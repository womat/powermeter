package fritzbox

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
	"unicode/utf16"
)

const (
	// DefaultSid is an invalid session in order to perform and
	// identify logouts.
	DefaultSid = "0000000000000000"
	// DefaultExpires is the amount of time of inactivity before
	// the FRITZ!Box automatically closes a session.
	DefaultExpires = 10 * time.Minute
)

var (
	// ErrInvalidCred is the error returned by Auth when
	// login attempt is not successful.
	ErrInvalidCred = errors.New("fritzbox: invalid credentials")

	// ErrExpiredSess means that client was too long inactive.
	ErrExpiredSess = errors.New("fritzbox: session expired")
)

// Session represents a FRITZ!Box session
type Session struct {
	client *Client

	XMLName   xml.Name      `xml:"SessionInfo"`
	Sid       string        `xml:"SID"`
	Challenge string        `xml:"Challenge"`
	BlockTime time.Duration `xml:"BlockTime"`

	// Rights' representation is a little bit tricky
	// TODO: Write UnmarshalXML to merge them
	RightsName   []string `xml:"Rights>Name"`
	RightsAccess []int8   `xml:"Rights>Access"`

	// Session expires after 10 minutes
	Expires time.Time `xml:"-"`
}

// NewSession returns a new FRITZ!Box session.
func NewSession(c *Client) *Session {
	return &Session{
		Sid:    DefaultSid,
		client: c,
	}
}

// Open retrieves the challenge from FRITZ!Box.
func (s *Session) Open() error {
	req, err := s.client.NewRequest("GET", "login_sid.lua", nil)
	if err != nil {
		return err
	}

	_, err = s.client.Do(req, s)
	if err != nil {
		return err
	}

	return nil
}

// Auth sends the Response (Challenge-Response) to the FRITZ!Box and
// returns an error, if any.
func (s *Session) Auth(username, password string) (err error) {
	var cr string
	var req *http.Request
	if cr, err = computeResponse(s.Challenge, password); err != nil {
		return
	}

	if req, err = s.client.NewRequest("POST", "login_sid.lua", url.Values{"username": {username}, "response": {cr}}); err != nil {
		return
	}

	if _, err = s.client.Do(req, s); err != nil {
		return
	}

	// Is login attempt successful?
	if s.Sid == DefaultSid {
		return ErrInvalidCred
	}

	_ = s.Refresh()
	return
}

// Close closes a session
func (s *Session) Close() {
	s.Sid = DefaultSid
}

// IsExpired returns true if session is expired
func (s *Session) IsExpired() bool {
	return s.Expires.Before(time.Now())
}

// Refresh updates expires
func (s *Session) Refresh() error {
	if s.IsExpired() && (s.Expires != time.Time{}) {
		s.Close()
		return ErrExpiredSess
	}
	s.Expires = time.Now().Add(DefaultExpires)
	return nil
}

// ComputeResponse generates a response for challenge-response auth
// with the given challenge and secret. It returns the response and
// and an error, if any.
func computeResponse(challenge, secret string) (resp string, err error) {
	buf := new(bytes.Buffer)
	chars := utf16.Encode([]rune(fmt.Sprintf("%s-%s", challenge, secret)))

	for _, char := range chars {
		// According to AVM's technical notes: unicode codepoints
		// above 255 needs to be converted to "." (0x2e 0x00 in UTF-16LE)
		if char > 255 {
			char = 0x2e
		}

		if err = binary.Write(buf, binary.LittleEndian, char); err != nil {
			return
		}
	}

	h := md5.New()
	_, _ = io.Copy(h, buf)
	resp = fmt.Sprintf("%s-%x", challenge, h.Sum(nil))
	return
}

func (s *Session) String() string {
	return s.Sid
}
