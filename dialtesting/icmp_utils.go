package dialtesting

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"net"
	"runtime"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

const DefaultICMPTTL = 64

func ProbeICMP(timeout time.Duration, target string) (rtt time.Duration, success bool) {
	var (
		requestType icmp.Type
		replyType   icmp.Type
		icmpConn    *icmp.PacketConn
		start       time.Time = time.Now()
		rttStart    time.Time
		rb          []byte
		dstIPAddr   *net.IPAddr
		dst         net.Addr
		idUnknown   bool
		err         error
		wb          []byte
	)
	{
		dstIPAddr, _, err = chooseProtocol(timeout, "", true, target)
		if err != nil {
			logger.Error("Error resolving address", "err", err)
			return 0, false
		}

		srcIP := net.ParseIP("::")

		logger.Info("Creating socket")

		privileged := true
		// Unprivileged sockets are supported on Darwin and Linux only.
		tryUnprivileged := runtime.GOOS == "darwin" || runtime.GOOS == "linux"

		if dstIPAddr.IP.To4() == nil {
			requestType = ipv6.ICMPTypeEchoRequest
			replyType = ipv6.ICMPTypeEchoReply

			if tryUnprivileged {
				// "udp" here means unprivileged -- not the protocol "udp".
				icmpConn, err = icmp.ListenPacket("udp6", srcIP.String())
				if err != nil {
					logger.Debug("Unable to do unprivileged listen on socket, will attempt privileged", "err", err)
				} else {
					privileged = false
				}
			}

			if privileged {
				icmpConn, err = icmp.ListenPacket("ip6:ipv6-icmp", srcIP.String())
				if err != nil {
					logger.Error("Error listening to socket", "err", err)
					return
				}
			}
			defer icmpConn.Close()

			if err := icmpConn.IPv6PacketConn().SetControlMessage(ipv6.FlagHopLimit, true); err != nil {
				logger.Debug("Failed to set Control Message for retrieving Hop Limit", "err", err)
			}
		} else {
			requestType = ipv4.ICMPTypeEcho
			replyType = ipv4.ICMPTypeEchoReply

			if tryUnprivileged {
				icmpConn, err = icmp.ListenPacket("udp4", srcIP.String())
				if err != nil {
					logger.Debug("Unable to do unprivileged listen on socket, will attempt privileged", "err", err)
				} else {
					privileged = false
				}
			}

			if privileged {
				icmpConn, err = icmp.ListenPacket("ip4:icmp", srcIP.String())
				if err != nil {
					logger.Error("Error listening to socket", "err", err)
					return
				}
			}
			defer icmpConn.Close()

			if err := icmpConn.IPv4PacketConn().SetControlMessage(ipv4.FlagTTL, true); err != nil {
				logger.Debug("Failed to set Control Message for retrieving TTL", "err", err)
			}
		}

		dst = dstIPAddr
		if !privileged {
			dst = &net.UDPAddr{IP: dstIPAddr.IP, Zone: dstIPAddr.Zone}
		}

		var data []byte = []byte("ICMP testing")

		body := &icmp.Echo{
			ID:   icmpID,
			Seq:  int(getICMPSequence()),
			Data: data,
		}
		logger.Info("Creating ICMP packet", "seq", body.Seq, "id", body.ID)
		wm := icmp.Message{
			Type: requestType,
			Code: 0,
			Body: body,
		}

		wb, err = wm.Marshal(nil)
		if err != nil {
			logger.Error("Error marshalling packet", "err", err)
			return
		}

		logger.Infof("Writing out packet, %v", time.Since(start))
		rttStart = time.Now()

		_, err = icmpConn.WriteTo(wb, dst)
		if err != nil {
			logger.Warn("Error writing to socket", "err", err)
			return
		}

		// Reply should be the same except for the message type and ID if
		// unprivileged sockets were used and the kernel used its own.
		wm.Type = replyType
		// Unprivileged cannot set IDs on Linux.
		idUnknown = !privileged && runtime.GOOS == "linux"
		if idUnknown {
			body.ID = 0
		}
		wb, err = wm.Marshal(nil)
		if err != nil {
			logger.Error("Error marshalling packet", "err", err)
			return
		}

		if idUnknown {
			// If the ID is unknown (due to unprivileged sockets) we also cannot know
			// the checksum in userspace.
			wb[2] = 0
			wb[3] = 0
		}

		rb = make([]byte, 65536)
		deadline := time.Now().Add(timeout)
		err = icmpConn.SetReadDeadline(deadline)
		if err != nil {
			logger.Error("Error setting socket deadline", "err", err)
			return
		}

	}
	logger.Infof("Waiting for reply packets, %v", time.Since(start))
	for {
		var n int
		var peer net.Addr
		var err error
		logger.Infof("Reading from socket, %v", time.Since(start))
		if dstIPAddr.IP.To4() == nil {
			n, _, peer, err = icmpConn.IPv6PacketConn().ReadFrom(rb)
		} else {
			n, _, peer, err = icmpConn.IPv4PacketConn().ReadFrom(rb)
		}
		if err != nil {
			if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
				logger.Warn("Timeout reading from socket", "err", err)
				return
			}
			logger.Error("Error reading from socket", "err", err)
			continue
		}
		if peer.String() != dst.String() {
			logger.Warn("Received packet from unexpected source", "peer", peer)
			continue
		}
		if idUnknown {
			// Clear the ID from the packet, as the kernel will have replaced it (and
			// kept track of our packet for us, hence clearing is safe).
			rb[4] = 0
			rb[5] = 0
		}
		if idUnknown || replyType == ipv6.ICMPTypeEchoReply {
			// Clear checksum to make comparison succeed.
			rb[2] = 0
			rb[3] = 0
		}
		if bytes.Equal(rb[:n], wb) {
			rtt = time.Since(rttStart)
			logger.Info("Found matching reply packet, rtt: %v", rtt)
			return rtt, true
		}
	}
}

// Returns the IP for the IPProtocol and lookup time.
func chooseProtocol(timeout time.Duration, IPProtocol string, fallbackIPProtocol bool, target string) (ip *net.IPAddr, lookupTime float64, err error) {
	if IPProtocol == "ip6" || IPProtocol == "" {
		IPProtocol = "ip6"
	} else {
		IPProtocol = "ip4"
	}

	logger.Info("Resolving target address", "target", target, "ip_protocol", IPProtocol)
	resolveStart := time.Now()

	defer func() {
		lookupTime = time.Since(resolveStart).Seconds()
	}()

	resolver := &net.Resolver{}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if !fallbackIPProtocol {
		ips, err := resolver.LookupIP(ctx, IPProtocol, target)
		if err == nil {
			for _, ip := range ips {
				logger.Info("Resolved target address", "target", target, "ip", ip.String())
				return &net.IPAddr{IP: ip}, lookupTime, nil
			}
		}
		logger.Error("Resolution with IP protocol failed", "target", target, "ip_protocol", IPProtocol, "err", err)
		return nil, 0.0, err
	}

	ips, err := resolver.LookupIPAddr(ctx, target)
	if err != nil {
		logger.Error("Resolution with IP protocol failed", "target", target, "err", err)
		return nil, 0.0, err
	}

	// Return the IP in the requested protocol.
	var fallback *net.IPAddr
	for _, ip := range ips {
		switch IPProtocol {
		case "ip4":
			if ip.IP.To4() != nil {
				logger.Info("Resolved target address", "target", target, "ip", ip.String())
				return &ip, lookupTime, nil
			}

			// ip4 as fallback
			fallback = &ip

		case "ip6":
			if ip.IP.To4() == nil {
				logger.Info("Resolved target address", "target", target, "ip", ip.String())
				return &ip, lookupTime, nil
			}

			// ip6 as fallback
			fallback = &ip
		}
	}

	// Unable to find ip and no fallback set.
	if fallback == nil || !fallbackIPProtocol {
		return nil, 0.0, fmt.Errorf("unable to find ip; no fallback")
	}

	logger.Info("Resolved target address", "target", target, "ip", fallback.String())
	return fallback, lookupTime, nil
}

func ProbeTest() {
	var (
		requestType icmp.Type
		replyType   icmp.Type
		icmpConn    *icmp.PacketConn
		start       time.Time = time.Now()
		rttStart    time.Time
		rb          []byte
		dstIPAddr   = &net.IPAddr{
			IP: net.ParseIP("37.152.148.45"),
		}
		dst       net.Addr
		idUnknown bool
		err       error
		wb        []byte
	)
	{
		srcIP := net.ParseIP("::")

		logger.Info("Creating socket")

		privileged := true
		// Unprivileged sockets are supported on Darwin and Linux only.
		tryUnprivileged := runtime.GOOS == "darwin" || runtime.GOOS == "linux"

		if dstIPAddr.IP.To4() == nil {
			requestType = ipv6.ICMPTypeEchoRequest
			replyType = ipv6.ICMPTypeEchoReply

			if tryUnprivileged {
				// "udp" here means unprivileged -- not the protocol "udp".
				icmpConn, err = icmp.ListenPacket("udp6", srcIP.String())
				if err != nil {
					logger.Debug("Unable to do unprivileged listen on socket, will attempt privileged", "err", err)
				} else {
					privileged = false
				}
			}

			if privileged {
				icmpConn, err = icmp.ListenPacket("ip6:ipv6-icmp", srcIP.String())
				if err != nil {
					logger.Error("Error listening to socket", "err", err)
					return
				}
			}
			defer icmpConn.Close()

			if err := icmpConn.IPv6PacketConn().SetControlMessage(ipv6.FlagHopLimit, true); err != nil {
				logger.Debug("Failed to set Control Message for retrieving Hop Limit", "err", err)
			}
		} else {
			requestType = ipv4.ICMPTypeEcho
			replyType = ipv4.ICMPTypeEchoReply

			if tryUnprivileged {
				icmpConn, err = icmp.ListenPacket("udp4", srcIP.String())
				if err != nil {
					logger.Debug("Unable to do unprivileged listen on socket, will attempt privileged", "err", err)
				} else {
					privileged = false
				}
			}

			if privileged {
				icmpConn, err = icmp.ListenPacket("ip4:icmp", srcIP.String())
				if err != nil {
					logger.Error("Error listening to socket", "err", err)
					return
				}
			}
			defer icmpConn.Close()

			if err := icmpConn.IPv4PacketConn().SetControlMessage(ipv4.FlagTTL, true); err != nil {
				logger.Debug("Failed to set Control Message for retrieving TTL", "err", err)
			}
		}

		dst = dstIPAddr
		if !privileged {
			dst = &net.UDPAddr{IP: dstIPAddr.IP, Zone: dstIPAddr.Zone}
		}

		var data []byte = []byte("ICMP testing")

		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		body := &icmp.Echo{
			ID:   icmpID,
			Seq:  int(uint16(r.Intn(1 << 16))),
			Data: data,
		}
		logger.Info("Creating ICMP packet", "seq", body.Seq, "id", body.ID)
		wm := icmp.Message{
			Type: requestType,
			Code: 0,
			Body: body,
		}

		wb, err = wm.Marshal(nil)
		if err != nil {
			logger.Error("Error marshalling packet", "err", err)
			return
		}

		logger.Infof("[=====]Writing out packet, %v", time.Since(start))
		rttStart = time.Now()

		_, err = icmpConn.WriteTo(wb, dst)
		if err != nil {
			logger.Warn("Error writing to socket", "err", err)
			return
		}

		// Reply should be the same except for the message type and ID if
		// unprivileged sockets were used and the kernel used its own.
		wm.Type = replyType
		// Unprivileged cannot set IDs on Linux.
		idUnknown = !privileged && runtime.GOOS == "linux"
		if idUnknown {
			body.ID = 0
		}
		wb, err = wm.Marshal(nil)
		if err != nil {
			logger.Error("Error marshalling packet", "err", err)
			return
		}

		if idUnknown {
			// If the ID is unknown (due to unprivileged sockets) we also cannot know
			// the checksum in userspace.
			wb[2] = 0
			wb[3] = 0
		}

		rb = make([]byte, 65536)
		deadline := time.Now().Add(10 * time.Second)
		err = icmpConn.SetReadDeadline(deadline)
		if err != nil {
			logger.Error("Error setting socket deadline", "err", err)
			return
		}

	}
	logger.Infof("[=====]Waiting for reply packets, %v", time.Since(start))
	var count int
	for {
		var n int
		var peer net.Addr
		var err error
		logger.Infof("[=====]Reading from socket %d, %v", count, time.Since(start))
		if dstIPAddr.IP.To4() == nil {
			n, _, peer, err = icmpConn.IPv6PacketConn().ReadFrom(rb)
		} else {
			n, _, peer, err = icmpConn.IPv4PacketConn().ReadFrom(rb)
		}
		if err != nil {
			if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
				logger.Warn("[=====]Timeout reading from socket", "err", err)
				return
			}
			logger.Error("[=====]Error reading from socket", "err", err)
			continue
		}
		if peer.String() != dst.String() {
			logger.Warn("[=====]Received packet from unexpected source", "peer", peer)
			count++
			continue
		}
		if idUnknown {
			// Clear the ID from the packet, as the kernel will have replaced it (and
			// kept track of our packet for us, hence clearing is safe).
			rb[4] = 0
			rb[5] = 0
		}
		if idUnknown || replyType == ipv6.ICMPTypeEchoReply {
			// Clear checksum to make comparison succeed.
			rb[2] = 0
			rb[3] = 0
		}
		if bytes.Equal(rb[:n], wb) {
			logger.Infof("[=====]Found matching reply packet,rtt: %v", time.Since(rttStart))
			return
		}
	}
}
