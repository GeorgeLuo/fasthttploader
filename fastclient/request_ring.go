package fastclient

import (
	"container/ring"
	"github.com/prometheus/common/log"
	"io"
	"net/mail"
	"os"
	"path/filepath"
	"regexp"
	"sync"
)

// NextMessage uses the global ring buffer never run out of urls to return
func (r *MessagesRing) NextMessage() VadeRequestWrapper {
	r.globalMessagesRing = r.globalMessagesRing.Next()
	return r.globalMessagesRing.Value.(VadeRequestWrapper)
}

// Messages Ring is a thread-safe ring to hold necessary information to send requests to vade
type MessagesRing struct {
	mutex              *sync.Mutex
	ring               *ring.Ring
	messagesDirectory  string
	globalMessagesRing *ring.Ring
	Uri                string
}

func NewMessagesRing(uri string, directory string) MessagesRing {

	mr := MessagesRing{mutex: &sync.Mutex{}, Uri: uri}
	vadeRequestList := LoadMessagesFromDirectory(uri, directory)

	ring := ring.New(len(vadeRequestList))

	for vr, _ := range vadeRequestList {
		ring.Value = vr
		ring = ring.Next()
	}

	mr.ring = ring
	return mr
}

// LoadMessagesFromDirectory takes a directory with .eml files and per file pushes into a ring of VadeMessages
// for consumption
func LoadMessagesFromDirectory(uri string, directory string) []VadeRequestWrapper {

	var (
		err        error
		file       io.ReadCloser
	)

	// regex for .eml files
	emlRegex, err := regexp.Compile("^.+\\.(eml)$")
	if err != nil {
		log.Fatal(err)
	}

	var vadeRequestList []VadeRequestWrapper

	// starting at directory top level, per .eml file insert
	err = filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err == nil && emlRegex.MatchString(info.Name()) {
			// read the file
			log.Infof("LoadMessagesFromDirectory: reading file: %+v", path)
			file, err = os.Open(path)
			if err != nil {
				panic(err)
			}
			defer file.Close()

			m, err := mail.ReadMessage(file)
			if err != nil {
				log.Fatal(err)
			}

			header := m.Header
			log.Infof("Date: %s", header.Get("Date"))
			log.Infof("From: %s", header.Get("From"))
			log.Infof("To: %s", header.Get("To"))
			log.Infof("Subject: %s", header.Get("Subject"))

			// find necessary headers for vade request http header
			var bodyBuff []byte

			numBytes, err := io.ReadFull(m.Body, bodyBuff)
			log.Infof("bytes read:", numBytes)

			if err != nil {
				log.Fatal(err)
			}
			log.Infof("%s", string(bodyBuff))

			vrW := NewVadeRequest(uri, header.Get("Date"), header.Get("Date"), header.Get("Date"), []string{header.Get("To")}, bodyBuff)

			// curl -v -X POST 'http://0.0.0.0:8080/api/v1/scan'
			// -H 'X-Inet: 172.0.0.1' -H 'X-Helo: test.example.com'
			// -H 'X-Mailfrom: user@test.example.com'
			// -H 'X-Rcptto: test1@dest.example.com'
			// -H 'X-Rcptto: test2@dest.example.com'
			// -H 'X-Sanitize: true'
			// --data-binary "@/tmp/sample.eml"; echo

			vadeRequestList = append(vadeRequestList, vrW)
			return nil
		} else if err != nil {
			log.Fatalf("failed to load a eml file: filepath=%s, err=%s", path, err.Error())
		}

		// nothing to
		return filepath.SkipDir
	})

	return vadeRequestList
}
