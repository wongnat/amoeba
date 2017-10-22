package main

import (
    "os"
    "io"
    "fmt"
	"log"
    "amoeba/amoeba"
)

func main() {
    var url string
    var sha string
    var dir string

    args := os.Args
    n    := len(args)

    if n != 4 {
        fmt.Println("Usage: amoeba <build-dir> <ssh-url> <commit-sha>")
        return
    }

    url = args[1]
    sha = args[2]
    dir = args[3]

    a, err := amoeba.NewAmoeba(url, sha, dir)
    if err != nil {
		log.Fatal(err)
	}
    defer a.Close()

    outputs, err := a.Start()
	if err != nil {
		log.Fatal(err)
	}
	
    for _, output := range outputs {
        io.Copy(os.Stdout, output.Stdout)
    }

    errs, err := a.Wait()
	if err != nil {
		log.Fatal(err)
	}
	
    passed := true
    for _, err = range errs {
        if err != nil {
            passed = false
            break
        }
    }

    if passed {
        fmt.Println("PASSED")
    } else {
        fmt.Println("FAILED")
    }
}
