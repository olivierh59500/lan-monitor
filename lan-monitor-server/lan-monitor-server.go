package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

//VERSION of the program
var version = "undefined-autogenerated"

var globalScanRange string
var globalScanIntervall int

//Config data struct to read the config file
type Config struct {
	NMAPRange     string
	HTTPPort      int
	ScanIntervall int //seconds
}

//ReadConfig reads the config file
func ReadConfig(configfile string) Config {
	_, err := os.Stat(configfile)
	if err != nil {
		log.Fatal("Config file is missing: ", configfile)
	}

	var config Config
	if _, err := toml.DecodeFile(configfile, &config); err != nil {
		log.Fatal(err)
	}
	return config
}

func callNMAP() {
	log.Println("Starting nmap caller")
	var Counter = 1
	var tempScanFileName = "temp_scan.xml"
	var scanResultsFileName = "scan.xml"
	for {
		log.Println("Init NMAP scan no:", Counter)
		cmd := exec.Command("nmap", "-p", "22,80", "-oX", tempScanFileName, globalScanRange)
		cmd.Stdin = strings.NewReader("some input")
		var out bytes.Buffer
		cmd.Stdout = &out
		err := cmd.Run()
		if err != nil {
			log.Println(err)
		}
		log.Println("Scan no.", Counter, "complete")
		//log.Printf("in all caps: %q\n", out.String())
		Counter = Counter + 1

		//copy to the scan.xml
		r, err := os.Open(tempScanFileName)
		if err != nil {
			panic(err)
		}
		defer r.Close()

		w, err := os.Create(scanResultsFileName)
		if err != nil {
			panic(err)
		}
		defer w.Close()

		// do the actual work
		n, err := io.Copy(w, r)
		if err != nil {
			panic(err)
		}
		log.Printf("Scan results saved %v bytes\n", n)
		<-time.After(time.Duration(globalScanIntervall) * time.Second)
	}
}

func pageHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[1:]
	log.Println("URL path: " + path)

	//in case we have no path refer/redirect to index.html
	if len(path) == 0 {
		path = "index.html"
	}

	f, err := os.Open(path)
	if err == nil {
		Reader := bufio.NewReader(f)

		var contentType string

		if strings.HasSuffix(path, "css") {
			contentType = "text/css"
		} else if strings.HasSuffix(path, ".html") {
			contentType = "text/html"
		} else if strings.HasSuffix(path, ".js") {
			contentType = "application/javascript"
		} else if strings.HasSuffix(path, ".png") {
			contentType = "image/png"
		} else if strings.HasSuffix(path, ".svg") {
			contentType = "image/svg+xml"
		} else {
			contentType = "text/plain"
		}

		w.Header().Add("Content Type", contentType)
		Reader.WriteTo(w)
	} else {
		w.WriteHeader(404)
		fmt.Fprintln(w, "404 - Page not found"+http.StatusText(404))
	}
}

func main() {
	log.Println("Starting lan-monitor-server ver: " + version)

	//process the config
	//1st the config file is read and set parameters applied
	//2nd the command line parameters are interpreted,
	//if they are set they will overrule the config file
	//3rd if none of the above is applied the program reverts to the hardcoded defaults

	//defaults
	var config Config
	defaultConfigFileLocation := "/etc/lan-monitor.conf"
	config.HTTPPort = 8080
	config.NMAPRange = "192.168.1.1/24"
	config.ScanIntervall = 120 //seconds

	displayVersion := flag.Bool("version", false, "Prints the version number")
	cmdlineHTTPPort := flag.Int("port", config.HTTPPort, "HTTP port for the webserver")
	cmdlineNMAPScanRange := flag.String("range", config.NMAPRange, "The range NMAP should scan e.g. 192.168.1.1/24 it has to be nmap compatible")
	cmdlineScanIntervall := flag.Int("scan-rate", config.ScanIntervall, "The intervall of the scans in seconds")
	configFileLocation := flag.String("config-file", defaultConfigFileLocation, "Location of the config file")
	flag.Parse()

	//read the configfile
	config = ReadConfig(*configFileLocation)

	//if no range is defined in the config file
	if config.NMAPRange == "" {
		globalScanRange = *cmdlineNMAPScanRange
	} else {
		globalScanRange = config.NMAPRange
	}

	//if no port is defined in the config file
	if config.HTTPPort == 0 {
		config.HTTPPort = *cmdlineHTTPPort
	}

	//if no scan intervall is defined in the config file
	if config.ScanIntervall == 0 {
		globalScanIntervall = *cmdlineScanIntervall
	} else {
		globalScanIntervall = config.ScanIntervall
	}

	log.Println("Config - range:", globalScanRange, "port:", config.HTTPPort, "intervall:", globalScanIntervall, "sec")

	if *displayVersion == true {
		fmt.Println("Version: " + version)
		return
	}

	//changing working dir
	log.Println("Changing working dir to: ")
	err := os.Chdir("../www")
	if err != nil {
		log.Fatalln("Unable to switch working dir")
	}

	workingDir, _ := os.Getwd()
	log.Println("Dir:" + workingDir)

	//init the scanning routine
	go callNMAP()

	//starting the webserver
	http.HandleFunc("/", pageHandler)
	err = http.ListenAndServe(":"+strconv.Itoa(config.HTTPPort), nil)
	if err != nil {
		log.Println("Server error - " + err.Error())
	}
}
