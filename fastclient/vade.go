package fastclient

import (
	"github.com/valyala/fasthttp"
	"log"
	"strings"
)

// VadeRequestWrapper contains a request compatible for fasthttp clients
type VadeRequestWrapper struct {
	// underlying request
	Request *fasthttp.Request
	BodyDetail BodyDetail
}

type BodyDetail struct {
	// Size in bytes
	Size       int
	// If the request was headerless
	HasHeaders bool
}

// NewVadeRequest is a method to initialize a VadeRequest
// XHelo is a domain the email came from
// XInet is the IP address the email came from
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

	bD := BodyDetail{
		Size: len(body),
		HasHeaders: true}

	return VadeRequestWrapper{Request:&request, BodyDetail:bD}
}

func NewHeadlerlessVadeRequest(requestURI string, body []byte) VadeRequestWrapper {

	var request fasthttp.Request

	request.AppendBody(body)

	request.Header.SetRequestURI(requestURI)
	request.Header.SetMethod(strings.ToUpper("POST"))
	request.Header.Set("Connection", "keep-alive")

	request.Header.SetContentType("application/octet-stream")
	log.Printf("headerless")

	bD := BodyDetail{
		Size: len(body),
		HasHeaders: false}

	return VadeRequestWrapper{Request:&request, BodyDetail: bD}
}

