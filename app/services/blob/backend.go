package blob

import (
	"context"
	"errors"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
)

type Backend struct {
	Read         func(context.Context, *query.GetBlobByKey) error
	Write        func(context.Context, *cmd.StoreBlob) error
	Remove       func(context.Context, *cmd.DeleteBlob) error
	ListKeys     func(context.Context, *query.ListBlobs) error
	ScanMetadata func(context.Context, *query.ScanBlobMetadata) error
}

func (backend Backend) Register() {
	bus.AddHandler(backend.Get)
	bus.AddHandler(backend.Store)
	bus.AddHandler(backend.Delete)
	bus.AddHandler(backend.List)
	bus.AddHandler(backend.Scan)
}

func (backend Backend) Get(ctx context.Context, request *query.GetBlobByKey) error {
	request.Result = nil
	if err := AuthorizeRead(ctx, request); err != nil {
		return err
	}

	return backend.Read(ctx, request)
}

func (backend Backend) Store(ctx context.Context, request *cmd.StoreBlob) error {
	if err := ValidateKey(request.Key); err != nil {
		return err
	}
	EnsureAuthorizedPrefix(ctx, request.Key)

	return backend.Write(ctx, request)
}

func (backend Backend) Delete(ctx context.Context, request *cmd.DeleteBlob) error {
	if err := ValidateKey(request.Key); err != nil {
		return err
	}
	EnsureAuthorizedPrefix(ctx, request.Key)

	return backend.Remove(ctx, request)
}

func (backend Backend) List(ctx context.Context, request *query.ListBlobs) error {
	request.Result = nil
	request.Skipped = 0
	if err := ValidatePrefix(request.Prefix); err != nil {
		return err
	}
	EnsureAuthorizedPrefix(ctx, request.Prefix)

	if err := backend.ListKeys(ctx, request); err != nil {
		return err
	}

	valid := request.Result[:0]
	for _, key := range request.Result {
		if ValidateKey(key) != nil {
			request.Skipped++
			continue
		}

		valid = append(valid, key)
	}
	request.Result = valid
	return nil
}

func (backend Backend) Scan(ctx context.Context, request *query.ScanBlobMetadata) error {
	tenant, _ := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	if tenant == nil {
		return errors.New("blob inventory requires a tenant")
	}
	if request.BatchSize < 1 || request.BatchSize > 200 {
		return errors.New("blob inventory batch size must be between 1 and 200")
	}

	request.Skipped = 0
	scan := *request
	scan.Accept = func(files []dto.BlobMetadata, next string, complete bool) error {
		valid := files[:0]
		for _, file := range files {
			if ValidateKey(file.Key) != nil {
				request.Skipped++
				continue
			}

			valid = append(valid, file)
		}

		return request.Accept(valid, next, complete)
	}
	return backend.ScanMetadata(ctx, &scan)
}
