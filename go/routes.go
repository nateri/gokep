package kepler

import (
	//"bufio"
	"errors"
	//"flag"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

	//"github.com/gin-gonic/gin/render"
	//"github.com/op/go-logging"
	//"io"
	//"io/ioutil"
	//stdlog "log"
	"html/template"
	"net/http"

	//"net/url"
	//"os"
	//"os/exec"
	//"path/filepath"
	//"golang.org/x/net/html"
	"bytes"
	"fmt"

	//"github.com/PuerkitoBio/goquery"
	//"golang.org/x/net/html"
	//"bufio"
	//"encoding/csv"
	//"net"
	//"regexp"
	//"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
	//"time"
)

var (
//log = logging.MustGetLogger("gokep")
)

type BusinessType int
type ResponseFormat int

const (
	Corporation BusinessType = iota + 1
	LLC_LP
	EntityNumber
	Corp_LLC_LP
)

const (
	Json ResponseFormat = iota
	Csv
	Html
)

/////////////////////////////////////////////////////
/////////////////////////////////////////////////////
/////////////////////////////////////////////////////
/////////////////////////////////////////////////////
/////////////////////////////////////////////////////
// Route Handlers

/*
func HandleIgnore(c *gin.Context) {
	// Don't even respond ...
	c.Writer.WriteHeaderNow()
	_, _, _ = c.Writer.Hijack()
	log.Critical("[HandleIgnore]")
	c.Abort(-1)
	return
}
*/

func KeplerAll(c *gin.Context) {

	log.Critical("[KeplerAll]")

	in := struct {
		Name    string         `form:"Name" binding:"required"`
		Format  ResponseFormat `form:"Format"`
		Details bool           `form:"Details"`
	}{}

	if err := binding.Form.Bind(c.Request, &in); err != nil {
		SetHtmlHeaders(c.Writer.Header())
		WriteContentFromTemplate(c.Writer, "landing", gin.H{"Error": "Bind Failure"})
		return
	}

	name := Sanitize(in.Name)

	log.Critical("[KeplerAll] [%s] %+v", name, in)

	if 0 == len(name) {
		SetHtmlHeaders(c.Writer.Header())
		WriteContentFromTemplate(c.Writer, "landing", gin.H{"Error": "Invalid Name"})
		return
	}

	var listings []Listing

	corps, err_corp := ProcessCorporation(name)
	if nil == err_corp {
		listings = append(listings, corps...)
	}
	llclp, err_llclp := ProcessLlcLp(name)
	if nil == err_llclp {
		listings = append(listings, llclp...)
	}

	switch in.Format {
	case Html:
		SetHtmlHeaders(c.Writer.Header())
		WriteContentFromTemplate(c.Writer, "kepget_loopback", listings)
	case Csv:
		SetCsvHeaders(c.Writer.Header(), name+".csv")
		c.Data(200, "text/csv", ConvertToCsv(listings))
	case Json:
		c.JSON(200, gin.H{"Result": "Success", "Matches": listings})
	}
}

func KeplerLlc(c *gin.Context) {

	Name := c.Params.ByName("name")

	in := struct {
		Format ResponseFormat `form:"Format"`
	}{}

	err := c.Bind(&in)
	if nil != err {
		c.JSON(500, gin.H{"Result": "Incorrect Parameters"})
		return
	}

	log.Critical("[KeplerLlc] [%s: %+v]", Name, in)

	// Input validation
	if 0 == len(Name) {
		c.JSON(404, gin.H{"Result": "Not Found"})
		return
	}

	listings, err := ProcessLlcLp(Name)

	switch err {
	case nil:
		switch in.Format {
		case Csv:
			SetCsvHeaders(c.Writer.Header(), "test.csv")
			c.Data(200, "text/csv", ConvertToCsv_Brief(listings))
		default:
			c.JSON(200, gin.H{"Result": "Success", "Matches": listings})
		}

	default:
		c.JSON(500, gin.H{"Result": "Fail", "Err": err.Error()})
	}
}
func KeplerCorp(c *gin.Context) {

	Name := c.Params.ByName("name")

	in := struct {
		Format ResponseFormat `form:"Format"`
	}{}

	err := c.Bind(&in)
	if nil != err {
		c.JSON(500, gin.H{"Result": "Incorrect Parameters"})
		return
	}

	log.Critical("[KeplerCorp] [%s: %+v]", Name, in)

	// Input validation
	if 0 == len(Name) {
		c.JSON(404, gin.H{"Result": "Not Found"})
		return
	}

	listings, err := ProcessCorporation(Name)

	switch err {
	case nil:
		switch in.Format {
		case Csv:
			SetCsvHeaders(c.Writer.Header(), "test.csv")
			c.Data(200, "text/csv", ConvertToCsv_Brief(listings))
		default:
			c.JSON(200, gin.H{"Result": "Success", "Matches": listings})
		}

	default:
		c.JSON(500, gin.H{"Result": "Fail", "Err": err.Error()})
	}
}
func KeplerNumber(c *gin.Context) {

	Number := c.Params.ByName("number")

	in := struct {
		Format ResponseFormat `form:"Format"`
	}{}

	err := c.Bind(&in)
	if nil != err {
		c.JSON(500, gin.H{"Result": "Incorrect Parameters"})
		return
	}

	log.Critical("[KeplerNumber] [%s: %+v]", Number, in)

	// Input validation
	if 0 == len(Number) {
		c.JSON(404, gin.H{"Result": "Not Found"})
		return
	}

	//var err error = nil
	//var listings []Listing

	listings, err := ProcessEntityNumber(Number)

	switch err {
	case nil:
		switch in.Format {
		case Csv:
			SetCsvHeaders(c.Writer.Header(), "test.csv")
			c.Data(200, "text/csv", ConvertToCsv(listings))
		case Html:
			c.HTML(200, "kep_corp", listings)
		default:
			c.JSON(200, gin.H{"Result": "Success", "Matches": listings})
		}

	default:
		c.JSON(500, gin.H{"Result": "Fail", "Err": err.Error()})
	}

	return
}

// @TODO: Move this to separate file
func WriteTemplate(w *bytes.Buffer, in_template string, data interface{}) error {
	log.Error("[WriteTemplate: %s]", in_template)

	t, err := template.ParseGlob("html/*")
	if err != nil {
		log.Debug("[Parse failed: %+v]", err)
		return err
	}

	t = t.Lookup(in_template)
	if t == nil {
		log.Debug("[Lookup failed]")
		return errors.New("Template Not Found")
	}

	err = t.Execute(w, data)
	if err != nil {
		log.Debug("[Execute failed: %+v]", err)
		return err
	}

	return nil
}

func WriteContentFromTemplate(w http.ResponseWriter, in_template string, data interface{}) {
	log.Debug("[WriteContentFromTemplate] [%s]", in_template)

	var page bytes.Buffer

	if err := WriteTemplate(&page, in_template, data); err != nil {
		log.Error("[RenderHtml] [ExecuteTemplate failed] %+v", data)
		// Render some error page
		fmt.Fprintf(w, "<!DOCTYPE html PUBLIC \"-//W3C//DTD XHTML 1.0 Transitional//EN\" \"http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd\">\n")
		fmt.Fprintf(w, "<html xmlns=\"http://www.w3.org/1999/xhtml\">\n")
		fmt.Fprintf(w, "<body>\n")
		fmt.Fprintf(w, "<p>Help! We're being repressed!<p>\n")
		fmt.Fprintf(w, "</body>\n")
		fmt.Fprintf(w, "</html>\n")
		return
	}

	w.Write(page.Bytes())
}

func Sanitize(name string) string {
	return stripchars(name)
}
func HandleKepler(c *gin.Context) {

	in := struct {
		Name    string         `form:"Name" binding:"required"`
		Type    BusinessType   `form:"Type"`
		Format  ResponseFormat `form:"Format"`
		Details bool           `form:"Details"`
	}{}

	if err := binding.Form.Bind(c.Request, &in); err != nil {
		//c.JSON(500, gin.H{"Result": "Incorrect Parameters"})
		SetHtmlHeaders(c.Writer.Header())
		result := gin.H{"Result": "Template Bind Failure"}
		WriteContentFromTemplate(c.Writer, "kepget_loopback", result)
		//c.Writer.Header().WriteHeader(code)
		return
	}

	name := Sanitize(in.Name)

	log.Critical("[HandleKepler] [%+v] %s", in, name)

	//render := NewRender(in.Format)

	//// Input validation
	//if 0 == len(in.Name) {
	//	render.Err(errors.err("Not Found"))
	//	return
	//}

	//switch in.Type {
	//case Corporation:
	//	corp := NewCorp(in.Name)
	//	result := corp.Query(in.Details)
	//}

	//render.Success(result)

	var err error = nil
	var listings []Listing

	switch in.Type {
	case Corporation:
		//corp := NewCorp(in.Name)
		//listings, err = corp.GetListings()
		listings, err = ProcessCorporation(in.Name)
	case LLC_LP:
		listings, err = ProcessLlcLp(in.Name)

	case Corp_LLC_LP:
		corps, err_corp := ProcessCorporation(in.Name)
		if nil == err_corp {
			listings = append(listings, corps...)
		}
		llclp, err_llclp := ProcessLlcLp(in.Name)
		if nil == err_llclp {
			listings = append(listings, llclp...)
		}

	case EntityNumber:
		listings, err = ProcessEntityNumber(in.Name)

	default:
		err = errors.New("Unknown Business Type")
	}

	switch err {
	case nil:
		switch in.Format {
		case Csv:
			SetCsvHeaders(c.Writer.Header(), name+".csv")
			c.Data(200, "text/csv", ConvertToCsv(listings))
		default:
			c.JSON(200, gin.H{"Result": "Success", "Matches": listings})
		}
	default:
		c.JSON(500, gin.H{"Result": "Fail", "Err": err.Error()})
	}
}

func HandleKeplers(c *gin.Context) {

	in := struct {
		Name    string         `form:"Name" binding:"required"`
		Type    BusinessType   `form:"Type"`
		Format  ResponseFormat `form:"Format"`
		Details bool           `form:"Details"`
	}{}

	if err := binding.Form.Bind(c.Request, &in); err != nil {
		//c.JSON(500, gin.H{"Result": "Incorrect Parameters"})
		SetHtmlHeaders(c.Writer.Header())
		result := gin.H{"Result": "Template Bind Failure"}
		WriteContentFromTemplate(c.Writer, "kepget_loopback", result)
		//c.Writer.Header().WriteHeader(code)
		return
	}

}

/////////////////////////////////////////////////////
/////////////////////////////////////////////////////
/////////////////////////////////////////////////////
/////////////////////////////////////////////////////
/////////////////////////////////////////////////////
// Create Router Paths

func AddGroups(r *gin.Engine) {

	//
	// Define Kepler API
	//

	// Retrieves a detailed listing for Business Number
	gNum := r.Group("/number/")
	gNum.GET("/:number", KeplerNumber)
	gNum.GET("/:number/*etc", KeplerNumber)

	// Retrieves a listing of Business Numbers
	gLlc := r.Group("/llc/")
	gLlc.GET("/:name", KeplerLlc)
	gLlc.GET("/:name/*etc", KeplerLlc)

	gCorp := r.Group("/corp/")
	gCorp.GET("/:name", KeplerCorp)
	gCorp.GET("/:name/*etc", KeplerCorp)

	gAll := r.Group("/all/")
	gAll.GET("/*etc", KeplerAll)

	// Web Portal
	r.GET("/kep", HandleKepler)
	r.POST("/keps", HandleKeplers)
}

func stripchars(str string) string {
	space, _ := utf8.DecodeLastRuneInString(" ")
	return strings.Map(func(r rune) rune {
		if unicode.IsDigit(r) || unicode.IsLetter(r) || space == r {
			return r
		}
		return -1
	}, str)
}
