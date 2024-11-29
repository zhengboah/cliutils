
class Headers {
  constructor(headerString) {
    let headers = {}
    try {
      headers = JSON.parse(headerString)
    } catch (error) {
      headers = {}
    }

    for (let k in headers) {
      this[k] = headers[k];
    }
  }

  get(name) {
    return this[name];
  }
}


class Response {
  constructor(statusCode, headers, body) {
    this.statusCode = statusCode;
    this.headers = new Headers(headers);
    this.body = body;
  }

  getResponseBody() {
    return this.body;
  }

  getStatusCode() {
    return this.statusCode;
  }

  getHeaders() {
    return this.headers;
  }
}

class API {
  constructor() {
    this.values = {}
    this.is_failed = false
    this.error_message = ""
  }

  fail(message) {
    console.log("fail message")
    this.is_failed = true;
    this.error_message = message;
  }

  setValue(key, value) {
    this.values[key] = value;
  }

  getValue(key) {
    return this.values[key];
  }

  getValues() {
    return this.values;
  }
}

let api = new API();
function getResult(response, api) {
  return JSON.stringify({
    response: {
      ...response,
      headers: response.headers
    },
    api: api
  })
}