package main

import (
    "os"
    "io"
    "fmt"
    "amoeba/repo"
    "amoeba/utils"
    "amoeba/amoeba"
)

func main() {
    args := os.Args
    n := len(args)

    if n < 2 || n > 3 {
        fmt.Println("Usage: amoeba <ssh-url> <commit-sha>")
        return
    }

    url    := args[1]
    commit := args[2]

    a, err := amoeba.NewAmoeba(url, commit)
    utils.CheckError(err)
    defer a.Close()

    outputs := a.Start()
    for _, output := range outputs {
        io.Copy(os.Stdout, output.Stdout)
    }

    errs := a.Wait()

    success := true
    for _, err = range errs {
        if err != nil {
            success = false
            break
        }
    }

    msg := repo.ParseName(url) + " at commit " + commit
    if success {
        fmt.Println(msg + " PASSED")
    } else {
        fmt.Println(msg + " FAILED")
    }
}
