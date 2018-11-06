package cms

import (
	// "fmt"
	"log"
	"os"
	"testing"
	// "time"
)

var (
	logger *log.Logger
)

func init() {
	serverLogFile, _ := os.OpenFile("cms_test.log", os.O_RDWR|os.O_CREATE, 0666)
	logger = log.New(serverLogFile, "[cms_test] ", log.Lmicroseconds|log.Lshortfile)
}

func TestBasic1(t *testing.T) {

}
