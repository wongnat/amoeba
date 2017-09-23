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


// // Create ./groups/{group}
// groupPath := filepath.Join(groupsDir, group)
// if _, err := os.Stat(groupPath); os.IsNotExist(err) {
//     err = os.Mkdir(groupPath, 0777)
//     utils.CheckError(err)
// }
