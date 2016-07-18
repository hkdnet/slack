package slack

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

var HTTPClient = &http.Client{}

type WebResponse struct {
	Ok    bool      `json:"ok"`
	Error *WebError `json:"error"`
}

type WebError string

func (s WebError) Error() string {
	return string(s)
}

func fileUploadReq(path, fpath string, values url.Values) (*http.Request, error) {
	fullpath, err := filepath.Abs(fpath)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(fullpath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	wr := multipart.NewWriter(body)

	ioWriter, err := wr.CreateFormFile("file", filepath.Base(fullpath))
	if err != nil {
		wr.Close()
		return nil, err
	}
	bytes, err := io.Copy(ioWriter, file)
	if err != nil {
		wr.Close()
		return nil, err
	}
	// Close the multipart writer or the footer won't be written
	wr.Close()
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if bytes != stat.Size() {
		return nil, errors.New("could not read the whole file")
	}
	req, err := http.NewRequest("POST", path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", wr.FormDataContentType())
	req.URL.RawQuery = (values).Encode()
	return req, nil
}

func parseResponseBody(body io.ReadCloser, intf *interface{}, debug bool) error {
	response, err := ioutil.ReadAll(body)
	if err != nil {
		return err
	}

	// FIXME: will be api.Debugf
	if debug {
		logger.Printf("parseResponseBody: %s\n", string(response))
	}

	err = json.Unmarshal(response, &intf)
	if err != nil {
		return err
	}

	return nil
}

func postWithMultipartResponse(path string, filepath string, values url.Values, intf interface{}, debug bool) error {
	req, err := fileUploadReq(SLACK_API+path, filepath, values)
	resp, err := HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return parseResponseBody(resp.Body, &intf, debug)
}

func postForm(endpoint string, values url.Values, intf interface{}, debug bool) error {
	resp, err := HTTPClient.PostForm(endpoint, values)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return parseResponseBody(resp.Body, &intf, debug)
}

func post(path string, values url.Values, intf interface{}, debug bool) error {
	return postForm(SLACK_API+path, values, intf, debug)
}

func parseAdminResponse(method string, teamName string, values url.Values, intf interface{}, debug bool) error {
	endpoint := fmt.Sprintf(SLACK_WEB_API_FORMAT, teamName, method, time.Now().Unix())
	return postForm(endpoint, values, intf, debug)
}

// IntOrString represetns a value whose type is string or int.
// Sometimes slack api response is type ambiguous...
type IntOrString struct {
	Kind   IntstrKind
	IntVal int64
	StrVal string
}

// IntstrKind represents the stored type of IntOrString.
type IntstrKind int

const (
	IntstrInt    IntstrKind = iota // The IntOrString holds an int.
	IntstrString                   // The IntOrString holds a string.
)

func (id *IntOrString) UnmarshalJSON(b []byte) error {
	var i int64
	if err := json.Unmarshal(b, &i); err == nil {
		id.Kind = IntstrInt
		id.IntVal = i
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		id.Kind = IntstrString
		id.StrVal = s
		return nil
	}
	return errors.New(`cannot convert string or nil`)
}
