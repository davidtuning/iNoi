package fs

import (
	"context"
	"fmt"
	"time"

	"github.com/OpenListTeam/OpenList/v4/internal/conf"
	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/errs"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
	"github.com/OpenListTeam/OpenList/v4/internal/task"
	"github.com/OpenListTeam/tache"
	"github.com/pkg/errors"
)

type UploadTask struct {
	task.TaskExtension
	storage          driver.Driver
	dstDirActualPath string
	file             model.FileStreamer
}

func (t *UploadTask) GetName() string {
	return fmt.Sprintf("上传 %s 到: [%s](%s)", t.file.GetName(), t.storage.GetStorage().MountPath, t.dstDirActualPath)
}

func (t *UploadTask) GetStatus() string {
	return "正在上传中..."
}

func (t *UploadTask) Run() error {
	t.ClearEndTime()
	t.SetStartTime(time.Now())
	defer func() { t.SetEndTime(time.Now()) }()
	return op.Put(t.Ctx(), t.storage, t.dstDirActualPath, t.file, t.SetProgress, true)
}

var UploadTaskManager *tache.Manager[*UploadTask]

// putAsTask add as a put task and return immediately
func putAsTask(ctx context.Context, dstDirPath string, file model.FileStreamer) (task.TaskExtensionInfo, error) {
	storage, dstDirActualPath, err := op.GetStorageAndActualPath(dstDirPath)
	if err != nil {
		return nil, errors.WithMessage(err, "存储获取失败")
	}
	if storage.Config().NoUpload {
		return nil, errors.WithStack(errs.UploadNotSupported)
	}
	if file.NeedStore() {
		_, err := file.CacheFullInTempFile()
		if err != nil {
			return nil, errors.Wrapf(err, "临时文件创建失败")
		}
		//file.SetReader(tempFile)
		//file.SetTmpFile(tempFile)
	}
	taskCreator, _ := ctx.Value(conf.UserKey).(*model.User) // taskCreator is nil when convert failed
	t := &UploadTask{
		TaskExtension: task.TaskExtension{
			Creator: taskCreator,
		},
		storage:          storage,
		dstDirActualPath: dstDirActualPath,
		file:             file,
	}
	t.SetTotalBytes(file.GetSize())
	UploadTaskManager.Add(t)
	return t, nil
}

// putDirect put the file and return after finish
func putDirectly(ctx context.Context, dstDirPath string, file model.FileStreamer, lazyCache ...bool) error {
	storage, dstDirActualPath, err := op.GetStorageAndActualPath(dstDirPath)
	if err != nil {
		_ = file.Close()
		return errors.WithMessage(err, "存储获取失败")
	}
	if storage.Config().NoUpload {
		_ = file.Close()
		return errors.WithStack(errs.UploadNotSupported)
	}
	return op.Put(ctx, storage, dstDirActualPath, file, nil, lazyCache...)
}
