package utils

import (
    "os"
)

// Check if the given dir exists and create it if it does not.
func CheckDir(dir string) {
    if _, err := os.Stat(dir); os.IsNotExist(err) {
        err = os.Mkdir(dir, 0777)
        CheckError(err)
    }
}

// Panics if the err is non-nil.
func CheckError(err error) {
    if err != nil {
        panic(err)
    }
}

// Catches the error that caused a panic and returns the err.
// TODO: haven't implemented error handling yet.
func HandleError() error {
    if err := recover(); err != nil {
        return err.(error)
    }

    return nil
}
