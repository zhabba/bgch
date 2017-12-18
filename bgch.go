package main

import (
	"fmt"
	"os"
	"flag"
	"log"
	"strings"
	"path/filepath"
	"os/exec"
	"time"
	"math/rand"
)

const BACKGROUND_DCONF_BASE_URI = "/org/gnome/desktop/background/"
const SCREENSAVER_DCONF_BASE_URI = "/org/gnome/desktop/screensaver/"
const KEY_PICTURE_URI = "picture-uri"
const KEY_PICTURE_OPTIONS = "picture-options" //zoom, whatever

var backgroundsDir = flag.String("bg", "~/Pictures", "Path to directory (or comma-separated list of directories) containing backgrounds. Optional")
var timeToShow = flag.Int("tts", 300, "Time to keep current background in seconds. Optional")
var changeLockScreen = flag.Bool("ls", false, "Change lockscreen background either. Optional")
var searchImagesRecursively = flag.Bool("r", false, "Search images recursively. Optional")
var needHelp = flag.Bool("help", false, "Show this help. Optional")

var userEnvironment = make(map[string]string)
var bgDirs = make([]string, 0)
var bgFiles = make([]string, 0)
var errors = make([]error, 0)
var allowedFileTypes = []string{"jpeg", "jpg", "png"}
var currentBg = ""

func init() {
	flag.Parse()
	if *needHelp != false {
		fmt.Println("Usage:")
		flag.PrintDefaults()
		os.Exit(0)
	}
}

func main() {
	readUserEnvironment()
	log.Printf("Will use %v dirs as backgrounds directory and change background every %v seconds.", *backgroundsDir, *timeToShow)
	setUpBackgroundsDir()
	go loop()
	select {}
}

func loop() {
	seed := rand.NewSource(time.Now().UnixNano())
	rnd := rand.New(seed)
	doneScan := make(chan bool, 1)
	bgDirIsEmpty := make(chan bool, 1)
	bgFileIsSingle := make(chan bool, 1)
	doneBgChange := make(chan bool, 1)
	doneScSaverChange := make(chan bool, 1)
	for {
		go scanBackgroundsDir(doneScan, bgDirIsEmpty, bgFileIsSingle)
		select {
		case <-bgDirIsEmpty:{
			log.Println("Bg dir is empty ... Place more nice images there ...")
			time.Sleep(time.Second * time.Duration(*timeToShow))
		}
		case <-bgFileIsSingle:{
			time.Sleep(time.Second * time.Duration(*timeToShow))
		}
		case <-doneScan:
			{
				var limit int
				if len(bgFiles) <= 1 {
					limit = 1
				} else {
					limit = len(bgFiles) -1
				}
				randInd := rnd.Intn(limit)
				bgFile := bgFiles[randInd]
				go changeBackground(bgFile, doneBgChange)
				go changeScreensaver(bgFile, doneScSaverChange)
				<-doneBgChange
				<-doneScSaverChange
				currentBg = bgFile
			}
		}
	}
}

func changeBackground(bg string, done chan bool) {
	if currentBg != bg && len(bgFiles) > 1 {
		execCommand(createBackgroundChangeCommand(bg))
		time.Sleep(time.Second * time.Duration(*timeToShow))
	}
	done <- true
}

func changeScreensaver(bg string, done chan bool) {
	if *changeLockScreen == true && len(bgFiles) > 1 {
		if currentBg != bg {
			execCommand(createScreensaverChangeCommand(bg))
			time.Sleep(time.Second * time.Duration(*timeToShow))
		}
	}
	done <- true
}

func createBackgroundChangeCommand(bg string) string {
	cmd := fmt.Sprintf("%v %v %v%v %v", "dconf", "write", BACKGROUND_DCONF_BASE_URI, KEY_PICTURE_URI, fmt.Sprintf("\"'file://%v'\"", bg))
	return cmd
}

func createScreensaverChangeCommand(bg string) string {
	cmd := fmt.Sprintf("%v %v %v%v %v", "dconf", " write", SCREENSAVER_DCONF_BASE_URI, KEY_PICTURE_URI, fmt.Sprintf("\"'file://%v'\"", bg))
	return cmd
}

func execCommand(cmd string) {
	chBgCmd := exec.Command("bash", "-c", cmd)
	out, err := chBgCmd.CombinedOutput()
	log.Printf("%v Out: %v, error: %v", cmd, string(out), err)
}

func readUserEnvironment() {
	for _, env := range os.Environ() {
		kv := strings.Split(env, "=")
		userEnvironment[kv[0]] = kv[1]
	}
}

func expandDirPath(path string) string {
	if strings.Contains(path[:2], "~/") {
		return filepath.Join(userEnvironment["HOME"], path[2:])
	}
	return path
}

func setUpBackgroundsDir() {
	if strings.Contains(*backgroundsDir, ",") {
		dirs := strings.Split(*backgroundsDir, ",")
		for _, dir := range dirs {
			if strings.Contains(dir[:2], "~/") {
				bgDirs = append(bgDirs, expandDirPath(dir))
			} else {
				bgDirs = append(bgDirs, dir)
			}
		}
	}
	bgDirs = append(bgDirs, expandDirPath(*backgroundsDir))
}

func scanBackgroundsDir(done chan bool, empty chan bool, single chan bool) {
	for _, root := range bgDirs {
		walkErr := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err == nil {
				if info.IsDir() && *searchImagesRecursively != true {
					if strings.Compare(path, root) == 0 {
						return err
					}
					return filepath.SkipDir
				} else {
					for _, t := range allowedFileTypes {
						if strings.Contains(strings.ToLower(info.Name()), t) && !func() bool {
							for _, f := range bgFiles {
								if f == path {
									return true
								}
							}
							return false
						}() {
							bgFiles = append(bgFiles, path)
						}
					}
				}
			}
			return err
		})
		if walkErr != nil {
			errors = append(errors, walkErr)
		}
	}
	log.Printf("Errors: %v", errors)
	if len(bgFiles) == 0 {
		empty <- true
	} else if len(bgFiles) == 1 {
		single <- true
	} else {
		done <- true
	}
}
