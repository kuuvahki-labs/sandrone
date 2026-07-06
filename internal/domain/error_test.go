package domain_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestAppErrorErrorAndUnwrap(t *testing.T) {
	require.Empty(t, (*domain.AppError)(nil).Error())
	require.Nil(t, (*domain.AppError)(nil).Unwrap())

	err := domain.NewError(domain.CodeInvalidArgument, "bad input")
	require.Equal(t, "invalid_argument: bad input", err.Error())
	require.Nil(t, err.Unwrap())

	cause := errors.New("boom")
	wrapped := domain.WrapError(domain.CodeParseFailed, "parse config", cause)
	require.Equal(t, "parse_failed: parse config: boom", wrapped.Error())
	require.ErrorIs(t, wrapped, cause)
	require.Equal(t, cause, wrapped.Unwrap())
}

func TestIsCode(t *testing.T) {
	require.True(t, domain.IsCode(domain.NewError(domain.CodeRenderFailed, "render"), domain.CodeRenderFailed))
	require.False(t, domain.IsCode(domain.NewError(domain.CodeRenderFailed, "render"), domain.CodeParseFailed))
	require.False(t, domain.IsCode(errors.New("plain"), domain.CodeRenderFailed))

	wrapped := errors.Join(errors.New("other"), domain.WrapError(domain.CodeFileMergeFailed, "merge", errors.New("cause")))
	require.True(t, domain.IsCode(wrapped, domain.CodeFileMergeFailed))
}
