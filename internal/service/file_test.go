package service_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

func TestServiceGetFileRequiresSpecOrName(t *testing.T) {
	svc := service.New()

	_, err := svc.GetFile(context.Background(), domain.FileRequest{})

	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeInvalidArgument))
}

func TestServiceGetFileWithoutProcessorsUsesSourceContent(t *testing.T) {
	svc := service.New()
	spec := domain.FileSpec{
		Name:   "x.txt",
		Kind:   domain.FileKindStatic,
		Source: domain.FileSource{Type: "inline", Content: "a: 1\n"},
	}

	result, err := svc.GetFile(context.Background(), domain.FileRequest{Spec: &spec})

	require.NoError(t, err)
	require.Equal(t, "a: 1\n", string(result.File.Content))
}
