package kepler

import (
	//"bufio"
	"errors"
	"flag"
	//"github.com/gin-gonic/gin"
	"github.com/op/go-logging"
	"io"
	"io/ioutil"
	//stdlog "log"
	"net/http"
	"net/url"
	//"os"
	//"os/exec"
	//"path/filepath"
	//"golang.org/x/net/html"
	"bytes"
	"github.com/PuerkitoBio/goquery"
	//"golang.org/x/net/html"
	//"bufio"
	"encoding/csv"
	//"net"
	"regexp"
	"strconv"
	"strings"
	//"time"
)

var (
	log       = logging.MustGetLogger("gokep")
	keplerUrl = flag.String("kepler_url_", "http://kepler.sos.ca.gov", "Url hosting Kepler")
)

type Agent struct {
	Name         string
	Address      string
	CityStateZip string
}
type Listing struct {
	Name         string
	Number       string
	DateFiled    string
	Status       string
	Jurisdiction string
	Address      string
	CityStateZip string
	Agent        Agent
	Type         BusinessType
}
type AspContext struct {
	EventTarget        string
	EventArgument      string
	ViewState          string
	ViewStateEncrypted string
	EventValidation    string
	Additional         string
}

func (this AspContext) ToString() string {
	var s string
	s += "__EVENTTARGET=" + this.EventTarget
	s += "&__EVENTARGUMENT=" + this.EventArgument
	s += "&__VIEWSTATE=" + this.ViewState
	s += "&__VIEWSTATEENCRYPTED=" + this.ViewStateEncrypted
	s += "&__EVENTVALIDATION=" + this.EventValidation
	s += this.Additional
	return s
}

func ProcessNumber(name string) ([]Listing, error) {
	// Sanitize Name?
	context := CreateContextQueryCorp(name)
	return getListingsForContext(context)
}
func ProcessCorporation(name string) ([]Listing, error) {
	// Sanitize Name?
	context := CreateContextQueryCorp(name)
	return getListingsForContext(context)
}
func ProcessLlcLp(name string) ([]Listing, error) {
	context := CreateContextQueryLlclp(name)
	return getListingsForContext(context)
}

/////////////////////////////////////////////////////
/////////////////////////////////////////////////////
/////////////////////////////////////////////////////
/////////////////////////////////////////////////////
/////////////////////////////////////////////////////
// Helpers

func getListingsForContext(context AspContext) ([]Listing, error) {

	reader, err := post(context)
	if nil != err {
		log.Error("[Post Failed] %+v", err)
		return []Listing{}, err
	}

	//switch format {
	//case ListFormatLong:
	//	shortListings, err := getShortListings(*reader)
	//default:
	//	listings, err := getDetailedListings(*reader)
	//}
	listings, err := getDetailedListingsFromPage(*reader)

	return listings, nil
}

func getDetailedListingsFromPage(r io.Reader) ([]Listing, error) {

	entNumList, err := getEntityNumbersFromSearchResultPage(r)
	if nil != err {
		log.Error("[Failed to get Entities] %+v", err)
		return []Listing{}, err
	}

	listings := []Listing{}

	for _, eNum := range entNumList {
		l, err := GetListing(eNum)
		if nil != err {
			log.Error("[Failed to get Details] %+v", err)
			l = Listing{
				Number: eNum,
				Status: "SCRIPT ERROR",
			}
		}
		listings = append(listings, l)
	}

	return listings, nil
}

func getEntityNumbersFromSearchResultPage(r io.Reader) ([]string, error) {
	entityNumbers := make([]string, 0, 5)

	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		log.Error("[Failed to create Document]")
		return entityNumbers, err
	}
	log.Debug("[Doc: %+v]", doc)

	// @TODO: Iterate through all pages
	//context := CreateContextFromPage(doc)

	doc.Find("tr").Has("td").Each(func(selIdx int, sel *goquery.Selection) {
		// For Each <tr>
		rowData := sel.Find("td")

		if rowData.Length() != 5 {
			h, _ := sel.Html()
			log.Debug("[Len: %d] %+v", rowData.Length(), h)
			return
		}

		entityNumbers = append(entityNumbers, rowData.Nodes[0].FirstChild.Data)
	})

	log.Debug("[EntityNumbers] %+v", entityNumbers)
	return entityNumbers, nil
}

func getListingFromEntitySearchResultPage(r io.Reader, bt BusinessType) (Listing, error) {

	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		log.Error("[Failed to create Document]")
		return Listing{}, err
	}
	log.Debug("[Doc: %+v]", doc)

	context := CreateContextFromPage(doc)

	// Update context for type
	switch bt {
	case Corporation:
		context.EventTarget = "ctl00%24content_placeholder_body%24SearchResults1%24GridView_SearchResults_Corp"
		context.EventArgument = "DetailCorp%240"
	case LLC_LP:
		context.EventTarget = "ctl00%24content_placeholder_body%24SearchResults1%24GridView_SearchResults_LPLLC"
		context.EventArgument = "DetailLPLLC%240"
	default:
		return Listing{}, errors.New("Unknown Business Type")
	}

	reader, err := post(context)
	if nil != err {
		log.Error("[Request failed] %+v", err)
		return Listing{}, err
	}

	return getListingFromEntityPage(*reader)
}

func getListingFromEntityPage(r io.Reader) (Listing, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		log.Error("[Failed to create Document]")
		return Listing{}, err
	}

	h, _ := doc.Html()
	log.Debug("[Details Page] %+v", h)

	listing := Listing{}

	doc.Find("tr").Has("td").Each(func(selIdx int, sel *goquery.Selection) {
		// For Each <tr>
		rowData := sel.Find("td")

		// Get the second <td> from each <tr>
		entry := rowData.Nodes[1].FirstChild.Data

		// Populate Listing according to table
		switch selIdx {
		case 0:
			listing.Name = entry
		case 1:
			listing.Number = entry
			listing.Type = Corporation
			if 'C' == listing.Number[0] {
				listing.Type = LLC_LP
			}
		case 2:
			listing.DateFiled = entry
		case 3:
			listing.Status = entry
		case 4:
			listing.Jurisdiction = entry
		case 5:
			listing.Address = entry
		case 6:
			listing.CityStateZip = entry
		case 7:
			sArr := regexp.MustCompile(" +").Split(entry, -1)
			listing.Agent.Name = strings.Join(sArr, " ")
		case 8:
			listing.Agent.Address = entry
		case 9:
			listing.Agent.CityStateZip = entry
		default:
		}
	})

	if 0 == len(listing.Number) {
		return Listing{}, errors.New("Entity Not Found")
	}

	return listing, nil
}

func CreateContextFromPage(doc *goquery.Document) AspContext {
	context := AspContext{}

	for _, attr := range doc.Find("#__VIEWSTATE").Nodes[0].Attr {
		if "value" == attr.Key {
			context.ViewState = url.QueryEscape(attr.Val)
			break
		}
	}
	for _, attr := range doc.Find("#__EVENTVALIDATION").Nodes[0].Attr {
		if "value" == attr.Key {
			context.EventValidation = url.QueryEscape(attr.Val)
			break
		}
	}

	return context
}

func GetListing(entityNumber string) (Listing, error) {
	context := CreateContextQueryEntity(entityNumber)

	businessType := LLC_LP
	if 'C' == entityNumber[0] {
		businessType = Corporation
	}

	reader, err := post(context)
	if nil != err {
		log.Error("[Request failed] %+v", err)
		return Listing{}, err
	}

	listing, err := getListingFromEntitySearchResultPage(*reader, businessType)
	if nil != err {
		log.Error("[Failed to get Listing] %+v", err)
		return Listing{}, err
	}

	log.Debug("[Listing: %+v]", listing)

	return listing, nil
}

func SetUrl(url string) {
	*keplerUrl = url
}

func post(context AspContext) (*io.Reader, error) {
	client := &http.Client{}

	req, err := http.NewRequest("POST", *keplerUrl, strings.NewReader(context.ToString()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 6.1; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/39.0.2171.95 Safari/537.36")

	resp, err := client.Do(req)
	if nil != err {
		log.Error("[Post failed] %+v", err)
		return nil, err
	}
	defer resp.Body.Close()

	log.Debug("[Response] %+v", resp)

	body, err := ioutil.ReadAll(resp.Body)
	if nil != err {
		log.Error("[Error Reading Body] %+v", err)
		return nil, err
	}

	var reader io.Reader
	reader = bytes.NewReader(body)

	return &reader, nil
}
func ConvertToCsv(listings []Listing) []byte {
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	w.UseCRLF = true
	for _, l := range listings {
		var record []string
		record = append(record, l.Name)
		record = append(record, l.Number)
		record = append(record, l.DateFiled)
		record = append(record, l.Status)
		record = append(record, l.Jurisdiction)
		record = append(record, l.Address)
		record = append(record, l.CityStateZip)
		record = append(record, l.Agent.Name)
		record = append(record, l.Agent.Address)
		record = append(record, l.Agent.CityStateZip)
		record = append(record, strconv.FormatInt(int64(l.Type), 10))
		w.Write(record)
	}
	w.Flush()
	return b.Bytes()
}
func ConvertToCsv_Brief(listings []Listing) []byte {
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	w.UseCRLF = true
	for _, l := range listings {
		var record []string
		record = append(record, l.Name)
		record = append(record, l.Number)
		record = append(record, l.DateFiled)
		record = append(record, l.Status)
		record = append(record, l.Agent.Name)
		record = append(record, strconv.FormatInt(int64(l.Type), 10))
		w.Write(record)
	}
	w.Flush()
	return b.Bytes()
}
func SetHtmlHeaders(headers http.Header) {
	headers.Set("Content-Type", "text/html")
}
func SetCsvHeaders(headers http.Header, filename string) {
	//header("Pragma: public")
	//header("Expires: 0")
	//header("Cache-Control: must-revalidate, post-check=0, pre-check=0")
	//header("Cache-Control: private",false)
	//header("Content-Type: application/octet-stream")
	//header("Content-Disposition: attachment; filename=\"$table.csv\";" )
	//header("Content-Transfer-Encoding: binary")
	headers.Set("Pragma", "public")
	headers.Set("Expires", "0")
	headers.Set("Cache-Control", "must-revalidate, post-check=0, pre-check=0")
	//headers.Set("Content-Type", "text/csv")
	//headers.Set("Content-Disposition", "attachment; filename=\"table.csv\";")

	//content_disposition := []string{"attachment;filename=", `"asdf"`}
	headers.Set("Content-Disposition", `attachment;filename="`+filename+`"`)
	headers.Set("Content-Transfer-Encoding", "binary")
}
