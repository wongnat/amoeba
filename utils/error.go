package utils

// Panics if the err is non-nil.
func CheckError(err error) {
    if err != nil {
        panic(err)
    }
}

// Catches the error that caused a panic and returns the err.
func HandleError() error {
    if err := recover(); err != nil {
        return err.(error)
    }

    return nil
}
