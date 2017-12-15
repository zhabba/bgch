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

var backgroundsDir = flag.String("bg", "Pictures", "Path to directory (or comma-separated list of directories) containing backgrounds. Optional")
var timeToShow = flag.Int("tts", 300, "Time to keep current background in seconds. Optional")
var changeLockScreen = flag.Bool("ls", false, "Change lockscreen background either. Optional")
var searchImagesRecursively = flag.Bool("r", false, "Search images recursively. Optional")
var needHelp = flag.Bool("help", false, "Show this help. Optional")

var userEnvironment = make(map[string]string)
var bgDirs []string
var bgFiles []string
var allowedFileTypes = []string{"jpeg", "jpg", "png"}

func init() {
	flag.Parse()
	if *needHelp != false {
		fmt.Println("Usage:")
		flag.PrintDefaults()
		os.Exit(0)
	}
}

func main() {
	var errors []error
	readUserEnvironment()
	log.Printf("Will use %v dirs as backgrounds directory and change background every %v seconds.", *backgroundsDir, *timeToShow)
	bgDirs = setUpBackgroundsDir(*backgroundsDir)
	bgFiles, errors = scanBackgroundsDir(bgDirs)
	log.Printf("Files %v ", bgFiles)
	startLoop(*timeToShow)
	if len(errors) > 0 {
		for _, e := range errors {
			log.Printf("Dir scan error: %v", e)
			os.Exit(1)
		}
	}
}

func startLoop(tts int) {
	seed := rand.NewSource(time.Now().UnixNano())
	rnd := rand.New(seed)
	for {
		randInd := rnd.Intn(len(bgFiles) - 1)
		bgFile := bgFiles[randInd]
		go changeBackground(bgFile)
		go changeScreensaver(bgFile)
		time.Sleep(time.Second * time.Duration(tts))
	}
}

func changeBackground(bg string) {
	execCommand(createBackgroundChangeCommand(bg))
}

func changeScreensaver(bg string) {
	if *changeLockScreen != true {
		return
	}
	execCommand(createScreensaverChangeCommand(bg))
}

func createBackgroundChangeCommand(bg string) string {
	cmd := fmt.Sprintf("%v %v %v%v %v", "dconf", "write", BACKGROUND_DCONF_BASE_URI, KEY_PICTURE_URI, fmt.Sprintf("\"'file://%v'\"", bg))
	log.Printf("command : %v", cmd)
	return cmd
}

func createScreensaverChangeCommand(bg string) string {
	cmd := fmt.Sprintf("%v %v %v%v %v", "dconf", " write", SCREENSAVER_DCONF_BASE_URI, KEY_PICTURE_URI, fmt.Sprintf("\"'file://%v'\"", bg))
	return cmd
}

func execCommand(cmd string) {
	chBgCmd := exec.Command("bash", "-c", cmd)
	out, err := chBgCmd.CombinedOutput()
	log.Printf("Out: %v, error: %v", string(out), err)
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

func setUpBackgroundsDir(bgDir string) []string {
	if strings.Contains(bgDir, ",") {
		dirs := strings.Split(bgDir, ",")
		for i, dir := range dirs {
			if strings.Contains(dir[:2], "~/") {
				dirs[i] = expandDirPath(dir)
			} else {
				dirs[i] = dir
			}
		}
		return dirs
	}
	return []string{expandDirPath(bgDir)}
}

func scanBackgroundsDir(paths []string) ([]string, []error) {
	files := make([]string, 0)
	hasErrors := make([]error, 0)
	for _, root := range paths {
		walkErr := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err == nil {
				if info.IsDir() && *searchImagesRecursively != true {
					if strings.Compare(path, root) == 0 {
						return err
					}
					return filepath.SkipDir
				} else {
					for _, t := range allowedFileTypes {
						if strings.Contains(strings.ToLower(info.Name()), t) {
							files = append(files, path)
						}
					}
				}
			}
			return err
		})
		if walkErr != nil {
			hasErrors = append(hasErrors, walkErr)
		}
	}
	return files, hasErrors
}
