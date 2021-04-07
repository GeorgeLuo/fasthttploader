package fastclient

import (
	"container/ring"
	"github.com/prometheus/common/log"
	"io"
	"io/ioutil"
   	"net/mail"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// NextMessage uses the global ring buffer never run out of urls to return
func (r *MessagesRing) NextMessage() VadeRequestWrapper {
	if r.ring == nil {
		log.Fatalf("r.ring is nil")
	}
	if r.ring.Next() == nil {
		log.Fatalf("r.ring.Next() is nil")
	}
	r.ring = r.ring.Next()
	return r.ring.Value.(VadeRequestWrapper)
}

// Messages Ring is a thread-safe ring to hold necessary information to send requests to vade
type MessagesRing struct {
	mutex             *sync.Mutex
	ring              *ring.Ring
	messagesDirectory string
	Uri               string
	UnderlyingData    []VadeRequestWrapper
	AvgSize           float64
	MaxSz             int
	MinSz             int
}

func NewMessagesRing(uri string, directory string, maxFileSize int, minFileSize int, sendHeaders bool) MessagesRing {

	log.Infof("NewMessagesRing: uri: %s, directory: %s", uri, directory)
	mr := MessagesRing{mutex: &sync.Mutex{}, Uri: uri}
	vadeRequestList, avgSize, max, min := LoadMessagesFromDirectory(uri, directory, maxFileSize, minFileSize, sendHeaders)

	log.Infof("NewMessagesRing: uri: %s, directory: %s, vadeRequestList=%d", uri, directory, len(vadeRequestList))
	ring := ring.New(len(vadeRequestList))
	mr.ring = ring
	mr.MaxSz = max
	mr.MinSz = min

	for _, vr := range vadeRequestList {
		ring.Value = vr
		ring = ring.Next()
	}

	mr.UnderlyingData = vadeRequestList
	mr.AvgSize = avgSize
	return mr
}

// LoadMessagesFromDirectory takes a directory with .eml files and per file pushes into a ring of VadeMessages
// for consumption
func LoadMessagesFromDirectory(uri string, directory string, maxFileSize int, minFileSize int, sendHeaders bool) (wrapperList []VadeRequestWrapper, avgSize float64, runningMaxFileSize int, runningMinFileSize int) {

	log.Infof("LoadMessagesFromDirectory: uri: %s, directory: %s", uri, directory)
	var (
		err  error
		file io.ReadCloser
	)

	// regex for .eml files
	msgRegex, err := regexp.Compile("^.+\\.(msg)$")
	if err != nil {
		log.Fatal(err)
	}

	// regex for .eml files
	emlRegex, err := regexp.Compile("^.+\\.(eml)$")
	if err != nil {
		log.Fatal(err)
	}

	// regex for .eml files
	mailRegex, err := regexp.Compile("^.+\\.(mail)$")
	if err != nil {
		log.Fatal(err)
	}

	var vadeRequestList []VadeRequestWrapper
	var runningTotal int

	// starting at directory top level, per .eml file insert
	err = filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err == nil && emlRegex.MatchString(info.Name()) || msgRegex.MatchString(info.Name()) || mailRegex.MatchString(info.Name()) {
			// read the file
			log.Infof("LoadMessagesFromDirectory: reading file: %+v", path)
			file, err = os.Open(path)
			if err != nil {
				panic(err)
			}

			defer file.Close()

			var fileBytes []byte
			fileBytes, err := ioutil.ReadFile(path)

			//log.Infof("fileBytes: %s", string(fileBytes))

			if len(fileBytes) > maxFileSize * 1000 || len(fileBytes) < minFileSize * 1000 {
				// skip this
				return nil
			}

			if runningMaxFileSize < len(fileBytes) {
				runningMaxFileSize = len(fileBytes)
			}

			if runningMinFileSize > len(fileBytes) {
				runningMinFileSize = len(fileBytes)
			}

			var mailStr string
			// this is from the aol dataset
			if mailRegex.MatchString(info.Name()) {
				mailStr = SanitizeMailFile(fileBytes)
				file = ioutil.NopCloser(strings.NewReader(mailStr))
			}

			m, err := mail.ReadMessage(file)
			if err != nil {
				log.Error(err)
				log.Infof(mailStr)
				if sendHeaders {
					// must send headers, so quit
					return nil
				}

				// if parsing fails, then we will leave the headers empty and just send the body as is
				vrW := NewHeadlerlessVadeRequest(uri, fileBytes)
				vadeRequestList = append(vadeRequestList, vrW)
				runningTotal += len(fileBytes)
				return nil
			}

			header := m.Header
			XInet, _ := ExtractXInet(header.Get("Received"))
			XHelo, _ := ExtractXHelo(header.Get("Received"))
			from := header.Get("From")
			to := header.Get("To")

			//log.Infof("From: %s", header.Get("From"))
			//log.Infof("To: %s", header.Get("To"))
			//log.Infof("Subject: %s", header.Get("Subject"))
			//log.Infof("XInet: %s", XInet)
			//log.Infof("XHelo: %s", XHelo)

			vrW := NewVadeRequest(uri, XInet, XHelo, from, []string{to}, fileBytes)

			// curl -v -X POST 'http://0.0.0.0:8080/api/v1/scan'
			// -H 'X-Inet: 172.0.0.1' -H 'X-Helo: test.example.com'
			// -H 'X-Mailfrom: user@test.example.com'
			// -H 'X-Rcptto: test1@dest.example.com'
			// -H 'X-Rcptto: test2@dest.example.com'
			// -H 'X-Sanitize: true'
			// --data-binary "@/tmp/sample.eml"; echo

			runningTotal += len(fileBytes)
			vadeRequestList = append(vadeRequestList, vrW)
			//return nil
		} else if err != nil {
			log.Fatalf("failed to load a eml file: filepath=%s, err=%s", path, err.Error())
		}

		// nothing to
		//return filepath.SkipDir
		return nil
	})

	return vadeRequestList, float64(runningTotal)/float64(len(vadeRequestList)), runningMaxFileSize, runningMinFileSize
}

// SanitizeMailFile processes an aol mail file which is preceded by smtp instructions:
// EHLO mailqa.office.aol.com
// MAIL FROM:<mailqauser75@aol.qa.testaol.com>
// RCPT TO:<aws01@aol.qa.test.aol.com>
// DATA
//
// and ends with:
// .
// QUIT
// we want to extract the EHLO
func SanitizeMailFile(fileBytes []byte) (fileStr string) {
	fileStr = string(fileBytes)

	fileStr = strings.ReplaceAll(fileStr, "__SENDER_ADDR__", "sender.address@gmail.com")
	fileStr = strings.ReplaceAll(fileStr, "__RECIP_ADDR__", "recip.address@gmail.com")

	lines := strings.Split(fileStr,"\n")

	endIndex := len(lines) - 2
	if endIndex <= 0 {
		return fileStr
	}
	fileStr = strings.Join(lines[4:endIndex], "\n")

	//XHelo, _ = GetStringFromMatchToEnd(lines[0], "EHLO")
	//from, _ = GetStringFromMatchToEnd(lines[1], "MAIL FROM:")
	//to, _ = GetStringFromMatchToEnd(lines[2], "RCPT TO:")

	return
}

// receivedHeader is a header that is formatted like this:
//     from 65.213.189.232  (HELO ocean07.youroptinmail.com) (65.213.189.232) by mta442.mail.yahoo.com with SMTP; 12 Feb 2003 16:41:18 -0800 (PST)
// we want the XInet value to be returned as 65.213.189.232
func ExtractXInet(receivedHeader string) (XInet string, found bool) {
	XInet, ok :=  GetStringInBetweenTwoString(receivedHeader, "from", "(HELO")
	if !ok {
		XInet, ok =  GetStringInBetweenTwoString(receivedHeader, "from", "(EHLO")
	}
	if !ok {
		log.Infof("ExtractXInet: %s", receivedHeader)
	}
	XInet = strings.TrimSpace(XInet)
	return XInet, ok
}

// receivedHeader is a header that is formatted like this:
//     from 65.213.189.232  (HELO ocean07.youroptinmail.com) (65.213.189.232) by mta442.mail.yahoo.com with SMTP; 12 Feb 2003 16:41:18 -0800 (PST)
// we want the XHelo value to be returned as ocean07.youroptinmail.com
func ExtractXHelo(receivedHeader string) (XHelo string, found bool) {
	XHelo, ok := GetStringInBetweenTwoString(receivedHeader, "(HELO", ")")
	if !ok {
		XHelo, ok = GetStringInBetweenTwoString(receivedHeader, "(EHLO", ")")
	}
	if !ok {
		log.Infof("ExtractXHelo: %s", receivedHeader)
	}
	return XHelo, ok
}

func GetStringInBetweenTwoString(str string, startS string, endS string) (result string, found bool) {
	s := strings.Index(str, startS)
	if s == -1 {
		return result,false
	}
	newS := str[s+len(startS):]
	e := strings.Index(newS, endS)
	if e == -1 {
		return result,false
	}
	result = newS[:e]
	return strings.TrimSpace(result),true
}

func GetStringFromMatchToEnd(str string, startS string) (result string, found bool) {
	s := strings.Index(str, startS)
	if s == -1 {
		return result,false
	}
	newS := str[s+len(startS):]
	return strings.TrimSpace(newS),true
}


