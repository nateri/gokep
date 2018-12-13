package main

import (
	//"bufio"
	//"errors"
	"flag"
	"gokep/go"

	"github.com/gin-gonic/gin"
	"github.com/op/go-logging"

	//"io"
	//"io/ioutil"
	stdlog "log"
	//"net/http"
	//"net/url"
	"os"
	//"os/exec"
	//"path/filepath"
	//"golang.org/x/net/html"
	//"bytes"
	//"github.com/PuerkitoBio/goquery"
	//"golang.org/x/net/html"
	//"bufio"
	//"encoding/csv"
	//"net"
	//"regexp"
	//"strconv"
	//"strings"
	"time"
)

var (
	log          = logging.MustGetLogger("gokep")
	keplerUrl    = flag.String("kepler_url", "http://kepler.sos.ca.gov", "Url hosting Kepler")
	httpListen   = flag.String("http", ":12346", "host:port to listen on")
	quit         = make(chan bool)
	sleepForQuit = 2
)

/////////////////////////////////////////////////////
/////////////////////////////////////////////////////
/////////////////////////////////////////////////////
/////////////////////////////////////////////////////
/////////////////////////////////////////////////////
// System Routers

func KeplerSys(c *gin.Context) {
	command := c.Params.ByName("command")
	log.Critical("[HandleSystem] [%v]", command)

	switch command {
	case "debug":
		{
			c.Writer.Write([]byte("Some debug info..."))
			c.Writer.Flush()
		}
		break
	case "shutdown":
		{
			c.Writer.Write([]byte("Shutting down..."))
			c.Writer.Flush()
			c.Request.Close = true
		}
		break
	}
}

/////////////////////////////////////////////////////
/////////////////////////////////////////////////////
/////////////////////////////////////////////////////
/////////////////////////////////////////////////////
/////////////////////////////////////////////////////
// Middleware

func SystemCb() gin.HandlerFunc {
	return func(c *gin.Context) {
		command := c.Params.ByName("command")

		switch command {
		case "shutdown":
			{
				log.Critical("[SystemCb:shutdown]")

				// Wait until all filters/handlers are done
				c.Next()

				// Notify quit
				close(quit)
			}
			break
		}
	}
}

func HandleIgnore(c *gin.Context) {
	// Don't even respond ...
	c.Writer.WriteHeaderNow()
	_, _, _ = c.Writer.Hijack()
	log.Critical("[HandleIgnore]")
	c.Abort(-1)
	return
}

/////////////////////////////////////////////////////
/////////////////////////////////////////////////////
/////////////////////////////////////////////////////
/////////////////////////////////////////////////////
/////////////////////////////////////////////////////
// Create Router Paths

func CreateHttpRouter() *gin.Engine {
	gin.SetMode(gin.DebugMode)

	// Creates a router without any middleware by default
	r := gin.New()

	// Global middlewares
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// Specify 404 (Not Found) Handler
	r.NoRoute(HandleIgnore)

	//
	// Attach Kepler Handler (Inject URL)
	kepler.SetUrl(*keplerUrl)
	kepler.AddGroups(r)

	//
	// Define System route
	gSys := r.Group("/sys/")
	gSys.Use(SystemCb())
	gSys.GET("/:command", KeplerSys)
	gSys.GET("/:command/*etc", KeplerSys)

	return r
}

/////////////////////////////////////////////////////
/////////////////////////////////////////////////////
/////////////////////////////////////////////////////
/////////////////////////////////////////////////////
/////////////////////////////////////////////////////
// Main

func main() {
	log.Critical("[main start]")
	defer func() { log.Critical("[main end]") }()

	//
	// Init
	flag.Parse()

	initLogging()

	startSysListener()

	router := CreateHttpRouter()
	//

	log.Critical("[Starting Service] [%s]", *httpListen)
	router.Run(*httpListen)
	//log.Fatal(http.ListenAndServe(*httpListen, router))
}

func initLogging() {
	// Customize the output format
	logging.SetFormatter(logging.MustStringFormatter("▶[%{level:.1s}] [%{time}] [%{module}]%{message}"))

	// Setup one stdout and one syslog backend.
	//logging.Level
	console_log := logging.NewLogBackend(os.Stderr, "", stdlog.LstdFlags|stdlog.Lshortfile)
	console_log.Color = false

	//var file_log_filter = syslog.LOG_DEBUG|syslog.LOG_LOCAL0|syslog.LOG_CRIT
	//log.Info("[%d]", file_log_filter)
	//file_log := logging.NewSyslogBackend("")
	//file_log.Color = false

	// Combine them both into one logging backend.
	logging.SetBackend(console_log)

	logging.SetLevel(logging.INFO, "gokep")
}
func startSysListener() {

	// Listen for async Quit request
	go func() {
		select {
		case <-quit:
			log.Critical("[Quit Gracefully]")
			os.Exit(1)
		}
		time.Sleep(time.Duration(sleepForQuit))
	}()

}
