package copyutil

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/samsarahq/go/oops"
)

// Methods taken from docker's CopyDirectory https://stackoverflow.com/a/56314145/6565736

func CopyDirectory(srcDir, dest string) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return oops.Wrapf(err, "read dir: %s", srcDir)
	}

	for _, entry := range entries {
		sourcePath := filepath.Join(srcDir, entry.Name())
		destPath := filepath.Join(dest, entry.Name())

		fileInfo, err := os.Stat(sourcePath)
		if err != nil {
			return oops.Wrapf(err, "stat: %s", sourcePath)
		}

		stat, ok := fileInfo.Sys().(*syscall.Stat_t)
		if !ok {
			return fmt.Errorf("failed to get raw syscall.Stat_t data for '%s'", sourcePath)
		}

		switch fileInfo.Mode() & os.ModeType {
		case os.ModeDir:
			if err := CreateIfNotExists(destPath, 0755); err != nil {
				return oops.Wrapf(err, "dest path: %s", destPath)
			}
			if err := CopyDirectory(sourcePath, destPath); err != nil {
				return oops.Wrapf(err, "copy dir: %s to %s", sourcePath, destPath)
			}
		case os.ModeSymlink:
			if err := CopySymLink(sourcePath, destPath); err != nil {
				return oops.Wrapf(err, "copy dir: %s to %s", sourcePath, destPath)
			}
		default:
			if err := Copy(sourcePath, destPath); err != nil {
				return oops.Wrapf(err, "copy dir: %s to %s", sourcePath, destPath)
			}
		}

		if err := os.Lchown(destPath, int(stat.Uid), int(stat.Gid)); err != nil {
			return oops.Wrapf(err, "")
		}

		fInfo, err := entry.Info()
		if err != nil {
			return oops.Wrapf(err, "")
		}

		isSymlink := fInfo.Mode()&os.ModeSymlink != 0
		if !isSymlink {
			if err := os.Chmod(destPath, fInfo.Mode()); err != nil {
				return oops.Wrapf(err, "")
			}
		}
	}
	return nil
}

func Copy(srcFile, dstFile string) error {
	out, err := os.Create(dstFile)
	if err != nil {
		return oops.Wrapf(err, "")
	}

	defer out.Close()

	in, err := os.Open(srcFile)
	if err != nil {
		return err
	}

	defer in.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return oops.Wrapf(err, "")
	}

	return nil
}

func Exists(filePath string) bool {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return false
	}

	return true
}

func CreateIfNotExists(dir string, perm os.FileMode) error {
	if Exists(dir) {
		return nil
	}

	if err := os.MkdirAll(dir, perm); err != nil {
		return fmt.Errorf("failed to create directory: '%s', error: '%s'", dir, err.Error())
	}

	return nil
}

func CopySymLink(source, dest string) error {
	link, err := os.Readlink(source)
	if err != nil {
		return err
	}
	return os.Symlink(link, dest)
}
