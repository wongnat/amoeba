package main

import (
    "os"
    "io"
    "fmt"
    "amoeba/repo"
    "amoeba/utils"
    "amoeba/lib"
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

    a, err := lib.NewAmoeba(url, sha, dir)
    utils.CheckError(err)
    defer a.Close()

    outputs := a.Start()
    for _, output := range outputs {
        io.Copy(os.Stdout, output.Stdout)
    }

    errs := a.Wait()

    passed := true
    for _, err = range errs {
        if err != nil {
            passed = false
            break
        }
    }

    msg := repo.ParseName(url) + " at commit " + sha
    if passed {
        fmt.Println(msg + " PASSED")
    } else {
        fmt.Println(msg + " FAILED")
    }
}
