package fastclient

import (
	"github.com/valyala/fasthttp"
	"strings"
)

// VadeRequestWrapper contains a request compatible for fasthttp clients
type VadeRequestWrapper struct {
	// underlying request
	Request *fasthttp.Request
}

func NewVadeRequest(requestURI string, XInet, XHelo, XMailFrom string, XRcptTo []string, body []byte) VadeRequestWrapper {

	var request fasthttp.Request

	request.AppendBody(body)
	request.Header.Add("X-Inet", XInet)
	request.Header.Add("X-Helo", XHelo)
	request.Header.Add("X-Mailfrom", XMailFrom)
	for _, rcpt := range XRcptTo {
		request.Header.Add("X-Rcptto", rcpt)
	}

	request.Header.SetRequestURI(requestURI)
	request.Header.SetMethod(strings.ToUpper("POST"))
	request.Header.Set("Connection", "keep-alive")

	request.Header.SetContentType("application/octet-stream")

	return VadeRequestWrapper{Request:&request}
}
