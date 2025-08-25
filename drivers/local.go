package drivers

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/google/uuid"
	"github.com/platship/go-storage"

	"github.com/platship/go-utils/osx"
)

type Local struct {
	Path       string `json:"path"`
	TmpPath    string `json:"tmpPath"`    // 临时路径
	PublicPath string `json:"publicPath"` // 公共路径
}

func NewLocal(path string) (*Local, error) {
	if path == "" {
		return nil, errors.New("path is undefined")
	}
	return &Local{Path: path}, nil
}

func (l *Local) Init() error {
	if l.Path == "" {
		return errors.New("path undefined")
	}
	return osx.CreateDirIsNotExist(l.Path, 0777)
}

func (l *Local) Close() error {
	return nil
}

func (l *Local) Get(key string) (*storage.GetValue, error) {
	if l.Path == "" {
		return nil, errors.New("path undefined")
	}
	f, err := os.Open(l.filePath(key))
	if err != nil {
		return nil, err
	}
	return storage.NewGetValue(f), nil
}

func (l *Local) Set(key string, val *storage.SetValue) (err error) {
	if l.Path == "" {
		return errors.New("path undefined")
	}
	path := l.filePath(key)
	_ = l.createDir(path)

	f, err := os.Create(path)
	if err != nil {
		return
	}
	defer f.Close()
	_, err = io.Copy(f, val.Reader)
	return
}

func (l *Local) Delete(key string) error {
	if l.Path == "" {
		return errors.New("path undefined")
	}
	path := l.filePath(key)
	if path == "" || path == "/" {
		return errors.New("path is empty or root dir")
	}
	return os.Remove(path)
}

func (l *Local) createDir(path string) error {
	dir, _ := filepath.Split(path)
	return osx.CreateDirIsNotExist(dir, 0777)
}

func (l *Local) filePath(key string) string {
	return filepath.Join(l.Path, key)
}

func (l *Local) ChunkInit() (string, error) {
	if err := os.MkdirAll(l.TmpPath, 0755); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(l.PublicPath), 0755); err != nil {
		return "", err
	}
	return uuid.New().String(), nil
}

func (l *Local) ChunkPart(uploadId string, partNumber int, reader io.Reader, size int64) (oss.UploadPart, error) {
	// 分片写入临时文件
	uploadDir := filepath.Join(l.TmpPath, uploadId)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return oss.UploadPart{}, fmt.Errorf("failed to create upload directory: %w", err)
	}
	partFilePath := filepath.Join(uploadDir, fmt.Sprintf("part-%d", partNumber))

	// 将分片数据写入本地文件
	outFile, err := os.Create(partFilePath)
	if err != nil {
		return oss.UploadPart{}, errors.New("failed to create part file: " + err.Error())
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, reader)
	if err != nil {
		return oss.UploadPart{}, errors.New("failed to write part file: " + err.Error())
	}

	// 对于本地存储，我们返回一个假的 oss.UploadPart，只包含分片序号
	return oss.UploadPart{PartNumber: partNumber}, nil
}

func (l *Local) ChunkComplete(uploadId string, parts []oss.UploadPart) error {
	uploadDir := filepath.Join(l.TmpPath, uploadId)
	finalFile, err := os.Create(l.PublicPath)
	if err != nil {
		return err
	}
	defer finalFile.Close()

	// 按照顺序读取所有分片并写入最终文件
	for i := 1; i <= len(parts); i++ {
		partFilePath := filepath.Join(uploadDir, fmt.Sprintf("part-%d", i))
		partFile, err := os.Open(partFilePath)
		if err != nil {
			return errors.New("failed to open part file: " + err.Error())
		}
		_, err = io.Copy(finalFile, partFile)
		partFile.Close()
		if err != nil {
			return errors.New("failed to copy part file: " + err.Error())
		}
	}
	// 合并后，清理分片目录
	return os.RemoveAll(uploadDir)
}

func (l *Local) ChunkAbort(uploadId string) error {
	uploadDir := filepath.Join(l.Path, uploadId)
	return os.RemoveAll(uploadDir)
}

func (l *Local) SetPath(path string, mores ...string) {
	l.Path = path
	if len(mores) == 2 {
		l.PublicPath = filepath.Join(mores[0], l.Path)
		l.TmpPath = filepath.Join(mores[1], filepath.Dir(l.Path))
	}
}

func (o *Local) GetPath() string {
	return o.Path
}
