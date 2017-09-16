package rest

import (
	"net/http"
	"mime/multipart"
	"fmt"
	"net"
	"net/url"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
)

type RestContext struct {
	httpClient *http.Client
	scheme string
	hostname string
	port int
	UserId string
	Password string
	setSessionId func(request *http.Request, id string)
}

type RestResponseType struct {
	HTTPStatusCode int
	HTTPReasonPhrase string
}

func NewRestResponseType(statusCode int, reason string) *RestResponseType {
	return &RestResponseType{
		HTTPStatusCode: statusCode,
		HTTPReasonPhrase: reason,
	}
}

func (b *RestResponseType) RestResponseTypeFieldsAsJSON() string {
	return fmt.Sprintf(
		"\"HTTPStatusCode\": %d, \"HTTPReasonPhrase\": \"%s\"",
		b.HTTPStatusCode, EncodeStringForJSON(b.HTTPReasonPhrase))
}

/*******************************************************************************
 * 
 */
type ParameterInfo struct {
	Name string
	Description string
}

func NewParameterInfo(name string, desc string) *ParameterInfo {
	return &ParameterInfo{
		Name: name,
		Description: desc,
	}
}

func (parameterInfo *ParameterInfo) AsJSON() string {
	return fmt.Sprintf(" {\"Name\": \"%s\", \"Description\": \"%s\"}",
		parameterInfo.Name, parameterInfo.Description)
}

/*******************************************************************************
 * For TCP/IP. userId and password are optional.
 */
func CreateTCPRestContext(scheme, hostname string, port int, userId string, password string,
	httpTransport *http.Transport, sessionIdSetter func(*http.Request, string)) *RestContext {

	var transport *http.Transport
	if httpTransport == nil {
		transport = &http.Transport{}
	} else {
		transport = httpTransport
	}

	return &RestContext{
		httpClient: &http.Client{
			Transport: transport,
			//CheckRedirect: func(req *http.Request, via []*http.Request) error { return nil },
		},
		scheme: scheme,
		hostname: hostname,
		port: port,
		UserId: userId,
		Password: password,
		setSessionId: sessionIdSetter,
	}
}

/*******************************************************************************
 * For Unix domain sockets. userId and password are optional.
 */
func CreateUnixRestContext(
	dialer func(network, addr string) (net.Conn, error),
	userId string, password string,
	sessionIdSetter func(*http.Request, string)) *RestContext {

	return &RestContext{
		httpClient: &http.Client{
			Transport: &http.Transport{
				Dial: dialer,
			},
		},
		scheme: "unix",
		hostname: "",
		port: 0,
		UserId: userId,
		Password: password,
		setSessionId: sessionIdSetter,
	}
}

func (restContext *RestContext) Print() {
	fmt.Println("RestContext:")
	fmt.Println(fmt.Sprintf("\thostname: %s", restContext.hostname))
	fmt.Println(fmt.Sprintf("\tport: %d", restContext.port))
}

func (restContext *RestContext) GetHttpClient() *http.Client { return restContext.httpClient }

func (restContext *RestContext) GetScheme() string { return restContext.scheme }

func (restContext *RestContext) GetHostname() string { return restContext.hostname }

func (restContext *RestContext) GetPort() int { return restContext.port }

func (restContext *RestContext) GetUserId() string { return restContext.UserId }

func (restContext *RestContext) GetPassword() string { return restContext.Password }

/*******************************************************************************
 * Send a GET request to the SafeHarborServer, at the specified REST endpoint method
 * (reqName), with the specified query parameters, using basic authentication.
 */
func (restContext *RestContext) SendBasicGet(reqName string) (*http.Response, error) {
	
	var urlstr string = restContext.getURL(true, reqName)
	
	var resp *http.Response
	var err error
	resp, err = restContext.httpClient.Get(urlstr)
	//resp, err = http.Get(urlstr)
	//var request *http.Request
	//request, err = http.NewRequest("GET", urlstr, nil)
	//if err != nil { return nil, err }
	//request.SetBasicAuth(restContext.UserId, restContext.Password)
	//resp, err = restContext.httpClient.Do(request)
	if err != nil { return nil, err }
	
	if err != nil { return nil, err }
	return resp, nil
}

/*******************************************************************************
 * Send a HEAD request to the SafeHarborServer, at the specified REST endpoint method
 * (reqName), with the specified query parameters, using basic authentication.
 */
func (restContext *RestContext) SendBasicHead(reqName string) (*http.Response, error) {
	
	var urlstr string = restContext.getURL(true, reqName)
	var resp *http.Response
	var err error
	resp, err = restContext.httpClient.Head(urlstr)
	if err != nil { return nil, err }
	return resp, nil
}

/*******************************************************************************
 * Send a DELETE request to the SafeHarborServer, at the specified REST endpoint method
 * (reqName), with the specified query parameters, using basic authentication.
 */
func (restContext *RestContext) SendBasicDelete(reqName string) (*http.Response, error) {
	
	var urlstr string = restContext.getURL(true, reqName)
	var resp *http.Response
	var request *http.Request
	var err error
	request, err = http.NewRequest("DELETE", urlstr, nil)
	if err != nil { return nil, err }
	resp, err = restContext.httpClient.Do(request)
	if err != nil { return nil, err }
	return resp, nil
}

/*******************************************************************************
 * Send a POST request to the SafeHarborServer, at the specified REST endpoint method
 * (reqName), with the specified query parameters, using basic authentication.
 */
func (restContext *RestContext) SendBasicFormPost(reqName string, names []string,
	values []string) (*http.Response, error) {
	
	var urlstr string = restContext.getURL(true, reqName)
	var data = make(map[string][]string)
	for i, value := range values { data[names[i]] = []string{value} }
	var resp *http.Response
	var err error
	var i = 0
	for {
		i++
		if i > 10 { return nil, errors.New("Too many redirects") }
		resp, err = restContext.httpClient.PostForm(urlstr, data)
		if err != nil { return nil, err }
		switch resp.StatusCode {
			case 200,201,202: return resp, nil
			case 301,302,303,307,308: 
				var newLocation = resp.Header["Location"][0]
				if newLocation == "" { return nil, errors.New("Empty location on redirect") }
				fmt.Println("Redirecting to " + newLocation)
				urlstr = newLocation
			default: return nil, errors.New(resp.Status)
		}
	}
	return resp, nil
}

/*******************************************************************************
 * Same as SendBasicFormPost, but called may specify custom request headers.
 */
func (restContext *RestContext) SendBasicFormPostWithHeaders(reqName string, names []string,
	values []string, headers map[string]string) (*http.Response, error) {
	
	if len(names) != len(values) { return nil, errors.New(
		"Number of names != number of values")
	}
	
	// Encode form name/values as an HTTP content stream.
	var data url.Values = make(map[string][]string)
	for i, name := range names {
		if len(name) == 0 { return nil, errors.New(
			"Zero length form parameter name")
		}
		data.Add(name, values[i])
	}
	var encodedData = data.Encode()

	var content io.Reader = strings.NewReader(encodedData)

	// Define the HTTP request object.
	var urlstr string = restContext.getURL(true, reqName)
	var request *http.Request
	var err error
	request, err = http.NewRequest("POST", urlstr, content)
	if err != nil { return nil, err }
	
	// Set HTTP headers on the request.
	if headers != nil {
		for name, value := range headers {
			request.Header.Set(name, value)
			fmt.Println(fmt.Sprintf("\theader: %s: %s", name, value))
		}
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Close = false
	
	// Perform the request.
	var response *http.Response
	response, err = restContext.httpClient.Do(request)
	if err != nil { return nil, err }
	return response, nil
}

/*******************************************************************************
 * Send request as a multi-part so that a file can be attached. Use basic authentication.
 */
func (restContext *RestContext) SendBasicFilePost(reqName string, names []string,
	values []string, path string) (*http.Response, error) {

	return restContext.SendFilePost("", reqName, names, values, path)
}

/*******************************************************************************
 * 
 */
func (restContext *RestContext) SendBasicStreamPost(reqName string, 
	headers map[string]string, content io.Reader) (*http.Response, error) {

	return restContext.SendBasicStreamReq("POST", reqName, headers, content)
}

/*******************************************************************************
 * 
 */
func (restContext *RestContext) SendBasicStreamPut(reqName string, 
	headers map[string]string, content io.Reader) (*http.Response, error) {

	return restContext.SendBasicStreamReq("PUT", reqName, headers, content)
}

/*******************************************************************************
 * Send a POST request with a body of an arbitrary content type.
 * The headers parameter may be nil.
 */
func (restContext *RestContext) SendBasicStreamReq(method string, reqName string, 
	headers map[string]string, content io.Reader) (*http.Response, error) {
	
	var url string = restContext.getURL(true, reqName)
	var request *http.Request
	var err error
	request, err = http.NewRequest(method, url, content)
	if err != nil { return nil, err }
	
	if headers != nil {
		for name, value := range headers {
			request.Header.Set(name, value)
		}
	}
	
	// Submit the request
	var response *http.Response
	fmt.Println("SendBasicStreamReq: url='" + url + "'")
	response, err = restContext.httpClient.Do(request)
	fmt.Println("SendBasicStreamReq: response Status='" + response.Status + "'")
	if err != nil { return nil, err }

	return response, nil
}

/*******************************************************************************
 * Send a GET request to the SafeHarborServer, at the specified REST endpoint method
 * (reqName), with the specified query parameters, using the specified session Id.
 */
func (restContext *RestContext) SendSessionGet(sessionId string, reqName string, names []string,
	values []string) (*http.Response, error) {

	return restContext.SendSessionReq(sessionId, "GET", reqName, names, values, nil, nil)
}

/*******************************************************************************
 * Send an HTTP POST formatted according to what is required by the SafeHarborServer
 * REST API, as defined in the slides "SafeHarbor REST API" of the design,
 * https://drive.google.com/open?id=1r6Xnfg-XwKvmF4YppEZBcxzLbuqXGAA2YCIiPb_9Wfo
 * Use the specified session Id.
 */
func (restContext *RestContext) SendSessionPost(sessionId string, reqName string, names []string,
	values []string) (*http.Response, error) {

	return restContext.SendSessionReq(sessionId, "POST", reqName, names, values, nil, nil)
}

/*******************************************************************************
 * Send an HTTP POST formatted according to what is required by the SafeHarborServer
 * REST API, as defined in the slides "SafeHarbor REST API" of the design,
 * https://drive.google.com/open?id=1r6Xnfg-XwKvmF4YppEZBcxzLbuqXGAA2YCIiPb_9Wfo
 */
func (restContext *RestContext) SendSessionReq(sessionId string, reqMethod string,
	reqName string, names []string, values []string, headerNames []string,
	headerValues []string) (*http.Response, error) {

	var urlstr string = restContext.getURL(true, reqName)
	var data url.Values = url.Values{}
	if names != nil {
		for index, each := range names {
			data[each] = []string{values[index]}
		}
	}
	var reader io.Reader = strings.NewReader(data.Encode())
	var request *http.Request
	var err error
	request, err = http.NewRequest(reqMethod, urlstr, reader)
	if err != nil { return nil, err }
	
	// Set custom headers, if any have been provided.
	var contentTypeIndex = -1
	if headerNames != nil {
		for index, headerName := range headerNames {
			if headerName == "Content-Type" {
				contentTypeIndex = index
			}
			request.Header.Set(headerName, headerValues[index])
		}
	}
	
	// For POSTs, if Content-Type not specified, set it to a default type of form-urlencoded.
	if (reqMethod == "POST") && (contentTypeIndex < 0) {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	
	if sessionId != "" {
		restContext.setSessionId(request, sessionId)
	}
	
	var resp *http.Response
	resp, err = restContext.httpClient.Do(request)
	if err != nil { return nil, err }
	return resp, nil
}

/*******************************************************************************
 * 
 */
func (restContext *RestContext) SendSessionStreamPost(sessionId string, reqMethod string,
	reqName string, dataToPost io.Reader, headerNames []string,
	headerValues []string) (*http.Response, error) {
	
	var urlstr string = restContext.getURL(true, reqName)
	
	// Add stream to requst.
	var request *http.Request
	var err error
	request, err = http.NewRequest(reqMethod, urlstr, dataToPost)
	if err != nil { return nil, err }
	
	// Set custom headers, if any have been provided.
	var contentTypeIndex = -1
	if headerNames != nil {
		for index, headerName := range headerNames {
			if headerName == "Content-Type" {
				contentTypeIndex = index
			}
			request.Header.Set(headerName, headerValues[index])
		}
	}
	
	// For POSTs, if Content-Type not specified, set it to a default type of form-urlencoded.
	if (reqMethod == "POST") && (contentTypeIndex < 0) {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	
	if sessionId != "" {
		restContext.setSessionId(request, sessionId)
	}
	
	
	
	// debug
	fmt.Print("url=")
	fmt.Println(request.URL)
	fmt.Println(request.Proto)
	fmt.Println(request.Header)
	fmt.Println(request.ContentLength)
	fmt.Println(request.Host)
	fmt.Println(request.PostForm)
	
	// end debug
	
	
	var resp *http.Response
	resp, err = restContext.httpClient.Do(request)
	if err != nil { return nil, err }
	return resp, nil
}

/*******************************************************************************
 * Send request as a multi-part so that a file can be attached.
 */
func (restContext *RestContext) SendSessionFilePost(sessionId string, reqName string, names []string,
	values []string, path string) (*http.Response, error) {

	return restContext.SendFilePost(sessionId, reqName, names, values, path)
}

/*******************************************************************************
 * Send request as a multi-part so that a file can be attached.
 */
func (restContext *RestContext) SendFilePost(sessionId string,
	reqName string, names []string,
	values []string, path string) (*http.Response, error) {

	var urlstr string = restContext.getURL(true, reqName)

	// Prepare a form that you will submit to that URL.
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	
	// Add file
	f, err := os.Open(path)
	if err != nil { return nil, err }
	var fileInfo os.FileInfo
	fileInfo, err = f.Stat()
	if err != nil { return nil, err }
	fw, err := w.CreateFormFile("filename", fileInfo.Name())
	if err != nil { return nil, err }
	_, err = io.Copy(fw, f)
	if err != nil { return nil, err }
	
	// Add the other fields
	if names != nil {
		for index, each := range names {
			fw, err = w.CreateFormField(each)
			if err != nil { return nil, err }
			_, err = fw.Write([]byte(values[index]))
			if err != nil { return nil, err }
		}
	}
	
	// Don't forget to close the multipart writer.
	// If you don't close it, your request will be missing the terminating boundary.
	w.Close()

	// Now that you have a form, you can submit it to your handler.
	req, err := http.NewRequest("POST", urlstr, &b)
	if err != nil { return nil, err }
	
	// Don't forget to set the content type, this will contain the boundary.
	req.Header.Set("Content-Type", w.FormDataContentType())
	if sessionId != "" { restContext.setSessionId(req, sessionId) }
	//if sessionId != "" { req.Header.Set("Session-Id", sessionId) }

	// Submit the request
	res, err := restContext.httpClient.Do(req)
	if err != nil { return nil, err }

	return res, nil
}

/*******************************************************************************
 * Parse an HTTP JSON response that can be converted to a map.
 */
func ParseResponseBodyToMap(body io.ReadCloser) (map[string]interface{}, error) {
	var value []byte
	var err error
	value, err = ioutil.ReadAll(body)
	if err != nil { return nil, err }
	var obj map[string]interface{}
	err = json.Unmarshal(value, &obj)
	if err != nil { return nil, err }
	return obj, nil
}

/*******************************************************************************
 * Parse an HTTP JSON response that can be converted to an array of maps.
 */
func ParseResponseBodyToMaps(body io.ReadCloser) ([]map[string]interface{}, error) {
	var value []byte
	var err error
	value, err = ioutil.ReadAll(body)
	if err != nil { return nil, err }
	var obj interface{}
	err = json.Unmarshal(value, &obj)
	if err != nil { return nil, err }
	
	var ar []interface{}
	var isType bool
	ar, isType = obj.([]interface{})
	if ! isType { return nil, errors.New(
		"Wrong type: obj is not a []interface{} - it is a " + 
			fmt.Sprintf("%s", reflect.TypeOf(obj)))
	}
	var maps = make([]map[string]interface{}, 0)
	for _, elt := range ar {
		var m map[string]interface{}
		m, isType = elt.(map[string]interface{})
		if ! isType { return nil, errors.New(
			"Wrong type: obj is not a []map[string]interface{} - it is a " + 
			fmt.Sprintf("%s", reflect.TypeOf(obj)))
		}
		maps = append(maps, m)
	}
	
	return maps, nil
}

/*******************************************************************************
 * Parse an HTTP JSON response that can be converted to an array of maps.
 * The response is assumed to consist of a single object with three fields:
 *	"HTTPStatusCode" - int
 *	"HTTPReasonPhrase" - string
 *	"payload" - json array (this is what is converted to a golang array of maps).
 */
func ParseResponseBodyToPayloadMaps(body io.ReadCloser) ([]map[string]interface{}, error) {
	
	fmt.Println("ParseResponseBodyToPayloadMaps: A")  // debug
	var value []byte
	var err error
	value, err = ioutil.ReadAll(body)
	fmt.Println("ParseResponseBodyToPayloadMaps: B")  // debug
	if err != nil { return nil, err }
	fmt.Println("ParseResponseBodyToPayloadMaps: C")  // debug
	var obj map[string]interface{}
	
	var s = string(value)
	var pos = strings.Index(s, "\x00")
	if pos >= 0 {
		fmt.Println(fmt.Sprintf("null char occurs at pos %d", pos))
		s = s[0:pos]
	}
	value = []byte(s)
	
	err = json.Unmarshal(value, &obj)
	fmt.Println("ParseResponseBodyToPayloadMaps: D")  // debug
	if err != nil {
		fmt.Print("ParseResponseBodyToPayloadMaps: D.1; value='")  // debug
		fmt.Print(string(value))
		fmt.Println("'")
		return nil, err
	}
	fmt.Println("ParseResponseBodyToPayloadMaps: E")  // debug
	
	var isType bool
	var httpStatusCode int
	var httpReasonPhrase string

	var f64 float64
	f64, isType = obj["HTTPStatusCode"].(float64)
	fmt.Println("ParseResponseBodyToPayloadMaps: F")  // debug
	if ! isType { return nil, errors.New("HTTPStatusCode is not an int: it is a " +
		reflect.TypeOf(obj["HTTPStatusCode"]).String()) }
	fmt.Println("ParseResponseBodyToPayloadMaps: G")  // debug
	httpStatusCode = int(f64)
	fmt.Println("ParseResponseBodyToPayloadMaps: H")  // debug
	if httpStatusCode != 200 {
		return nil, errors.New(fmt.Sprintf("HTTP status %s returned", httpStatusCode))
	}
	fmt.Println("ParseResponseBodyToPayloadMaps: I")  // debug

	httpReasonPhrase, isType = obj["HTTPReasonPhrase"].(string)
	fmt.Println("ParseResponseBodyToPayloadMaps: J")  // debug
	if httpReasonPhrase == "" { return nil, errors.New("No HTTPReasonPhrase") }
	fmt.Println("ParseResponseBodyToPayloadMaps: K")  // debug
	if ! isType { return nil, errors.New("HTTPReasonPhrase is not a string") }
	fmt.Println("ParseResponseBodyToPayloadMaps: L")  // debug

	var iar []interface{}
	iar, isType = obj["payload"].([]interface{})
	fmt.Println("ParseResponseBodyToPayloadMaps: M")  // debug
	if ! isType { return nil, errors.New("payload is not an array of interface") }
	fmt.Println("ParseResponseBodyToPayloadMaps: N")  // debug
	
	var maps = make([]map[string]interface{}, 0)
	fmt.Println("ParseResponseBodyToPayloadMaps: O")  // debug
	for _, elt := range iar {
		fmt.Println("ParseResponseBodyToPayloadMaps: O.1")  // debug
		var m map[string]interface{}
		m, isType = elt.(map[string]interface{})
		fmt.Println("ParseResponseBodyToPayloadMaps: O.2")  // debug
		if ! isType { return nil, errors.New("Element is not a map[string]interface") }
		fmt.Println("ParseResponseBodyToPayloadMaps: O.3")  // debug
		maps = append(maps, m)
		fmt.Println("ParseResponseBodyToPayloadMaps: O.4")  // debug
	}
	fmt.Println("ParseResponseBodyToPayloadMaps: P")  // debug
	
	return maps, nil
}

/*******************************************************************************
 * Write the specified map to stdout.
 */
func PrintMap(m map[string]interface{}) {
	fmt.Println("{")
	var firstTime bool = true
	for k, v := range m {
		if firstTime { firstTime = false } else { fmt.Println(",") }
		var s string
		var isString bool
		s, isString = v.(string)
		if isString {
			fmt.Print(fmt.Sprintf("\t\"%s\": \"%s\"", k, s))
		} else {
			fmt.Print(fmt.Sprintf("\t\"%s\": %v", k, v))
		}
	}
	fmt.Println()
	fmt.Println("}")
}

/*******************************************************************************
 * Write the specified maps to stdout.
 */
func PrintMaps(ms []map[string]interface{}) {
	fmt.Println("[")
	for i, m := range ms {
		if i > 0 { fmt.Println(",") }
		PrintMap(m)
	}
	fmt.Println("]")
}

/*******************************************************************************
 * If the response is not 200, then throw an exception.
 */
func (restContext *RestContext) Verify200Response(resp *http.Response) bool {
	var is200 bool = true
	if resp.StatusCode != 200 {
		is200 = false
		fmt.Sprintf("Response code %d", resp.StatusCode)
		var responseMap map[string]interface{}
		var err error
		responseMap, err = ParseResponseBodyToMap(resp.Body)
		if err == nil { PrintMap(responseMap) }
		//if restContext.stopOnFirstError { os.Exit(1) }
	}
	return is200
}

/*******************************************************************************
 * 
 */
func (restContext *RestContext) getURL(basicAuth bool, reqName string) string {
	var basicAuthCreds = ""
	if basicAuth {
		if restContext.UserId != "" {
			basicAuthCreds = fmt.Sprintf("%s:%s@", restContext.UserId, restContext.Password)
		}
	}
	var portspec = ""
	if restContext.port != 0 { portspec = fmt.Sprintf(":%d", restContext.port) }
	var httpScheme = restContext.GetScheme()
	var hostname = restContext.hostname
	if restContext.GetScheme() == "unix" {
		httpScheme = "http"  // override
		hostname = "fakehost.fak"
	}
	return fmt.Sprintf(
		"%s://%s%s%s/%s",
		httpScheme, basicAuthCreds, hostname, portspec, reqName)
}

/*******************************************************************************
 * 
 * Utility to encode an arbitrary string value, which might contain quotes and other
 * characters, so that it can be safely and securely transported as a JSON string value,
 * delimited by double quotes. Ref. http://json.org/.
 */
func EncodeStringForJSON(value string) string {
	// Replace each occurrence of double-quote and backslash with backslash double-quote
	// or backslash backslash, respectively.
	
	var encodedValue = value
	encodedValue = strings.Replace(encodedValue, "\\", "\\\\", -1)
	encodedValue = strings.Replace(encodedValue, "\"", "\\\"", -1)
	encodedValue = strings.Replace(encodedValue, "/", "\\/", -1)
	encodedValue = strings.Replace(encodedValue, "\b", "\\b", -1)
	encodedValue = strings.Replace(encodedValue, "\f", "\\f", -1)
	encodedValue = strings.Replace(encodedValue, "\n", "\\n", -1)
	encodedValue = strings.Replace(encodedValue, "\r", "\\r", -1)
	encodedValue = strings.Replace(encodedValue, "\t", "\\t", -1)
	return encodedValue
}

/*******************************************************************************
 * Reverse the encoding that is performed by EncodeStringForJSON.
 */
func DecodeStringFromJSON(encodedValue string) string {
	var decodedValue = encodedValue
	decodedValue = strings.Replace(decodedValue, "\\t", "\t", -1)
	decodedValue = strings.Replace(decodedValue, "\\r", "\r", -1)
	decodedValue = strings.Replace(decodedValue, "\\n", "\n", -1)
	decodedValue = strings.Replace(decodedValue, "\\f", "\f", -1)
	decodedValue = strings.Replace(decodedValue, "\\b", "\b", -1)
	decodedValue = strings.Replace(decodedValue, "\\/", "/", -1)
	decodedValue = strings.Replace(decodedValue, "\\\"", "\"", -1)
	decodedValue = strings.Replace(decodedValue, "\\\\", "\\", -1)
	return decodedValue
}

/*******************************************************************************
 * Write the specified byte array in JSON format.
 */
func ByteArrayAsJSON(bytes []byte) string {
	var s = "["
	for i, b := range bytes {
		if i > 0 { s = s + ", " }
		s = s + fmt.Sprintf("%d", b)
	}
	return (s + "]")
}

/*******************************************************************************
 * 
 */
func HttpOKResponse() string {
	return "\"HTTPStatusCode\": 200, \"HTTPReasonPhrase\": \"OK\""
}

/*******************************************************************************
 * 
 */
type ParameterValueDesc struct {
	Name string
	StringValue string
}

func NewParameterValueDesc(name string, strValue string) *ParameterValueDesc {
	return &ParameterValueDesc{
		Name: name,
		//Type: tp,
		StringValue: strValue,
	}
}
