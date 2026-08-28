package service

import (
	"mime"
	"strings"
	"unicode"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

const maxShareFilenameBaseRunes = 128

var subscriptionFilenameExtensions = []string{".txt", ".yaml", ".yml", ".json", ".conf"}

func sharePresentation(share domain.Share) domain.SharePresentation {
	switch normalizeFormat(share.TargetKind) {
	case "subscription":
		format := normalizeFormat(share.TargetFormat)
		if format == "" {
			format = "base64"
		}
		filenames := make(map[string]string, len(renderPresentations))
		for registeredFormat := range renderPresentations {
			filenames[registeredFormat] = shareResponseFilename(share, "", registeredFormat)
		}
		return domain.SharePresentation{
			PublicFilename:  shareResponseFilename(share, "", format),
			FormatFilenames: filenames,
		}
	case "file":
		return domain.SharePresentation{
			PublicFilename: stableFileShareFilename(share),
		}
	default:
		return domain.SharePresentation{PublicFilename: "share"}
	}
}

func shareResponseFilename(share domain.Share, finalFileName, format string) string {
	var filename string
	switch normalizeFormat(share.TargetKind) {
	case "subscription":
		filename = subscriptionShareFilename(share, format)
	case "file":
		filename = fileShareFilename(share, finalFileName)
	default:
		filename = "share"
	}
	if share.AgeRecipient != "" {
		filename = ageShareFilename(filename)
	}
	return filename
}

func stableFileShareFilename(share domain.Share) string {
	return shareResponseFilename(share, "", "")
}

func subscriptionShareFilename(share domain.Share, format string) string {
	presentation, _ := renderPresentationFor(format)
	for _, candidate := range []string{share.Name, share.TargetName, share.ID} {
		candidate = sanitizeShareFilenameForSuffix(candidate)
		candidate = stripSubscriptionFilenameExtensions(candidate)
		if base := truncateRunes(sanitizeShareFilename(candidate), maxShareFilenameBaseRunes); base != "" {
			return base + presentation.extension
		}
	}
	return "share" + presentation.extension
}

func fileShareFilename(share domain.Share, finalFileName string) string {
	for _, candidate := range []string{share.Name, share.TargetName, finalFileName, share.ID} {
		if filename := truncateFileShareFilename(sanitizeShareFilename(candidate)); filename != "" {
			return filename
		}
	}
	return "share"
}

func stripSubscriptionFilenameExtensions(name string) string {
	name, _ = trimSuffixFold(name, ".age")
	for _, extension := range subscriptionFilenameExtensions {
		if stripped, ok := trimSuffixFold(name, extension); ok {
			return stripped
		}
	}
	return name
}

func trimSuffixFold(value, suffix string) (string, bool) {
	if len(value) < len(suffix) || !strings.EqualFold(value[len(value)-len(suffix):], suffix) {
		return value, false
	}
	return value[:len(value)-len(suffix)], true
}

func sanitizeShareFilename(name string) string {
	return strings.TrimFunc(replaceUnsafeShareFilenameCharacters(name), trimShareFilenameBoundary)
}

func sanitizeShareFilenameForSuffix(name string) string {
	name = replaceUnsafeShareFilenameCharacters(name)
	name = strings.TrimLeftFunc(name, unicode.IsSpace)
	return strings.TrimRightFunc(name, trimShareFilenameBoundary)
}

func replaceUnsafeShareFilenameCharacters(name string) string {
	name = strings.ToValidUTF8(name, "�")
	var sanitized strings.Builder
	sanitized.Grow(len(name))
	for _, char := range name {
		if unicode.IsControl(char) || strings.ContainsRune(`<>:"/\|?*`, char) {
			sanitized.WriteRune('_')
			continue
		}
		sanitized.WriteRune(char)
	}
	return sanitized.String()
}

func trimShareFilenameBoundary(char rune) bool {
	return char == '.' || unicode.IsSpace(char)
}

func truncateFileShareFilename(filename string) string {
	if filename == "" {
		return ""
	}
	base, extension, found := strings.CutLast(filename, ".")
	if !found || base == "" {
		return truncateRunes(filename, maxShareFilenameBaseRunes)
	}
	return truncateRunes(base, maxShareFilenameBaseRunes) + "." + extension
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func ageShareFilename(filename string) string {
	for {
		withoutAge, ok := trimSuffixFold(filename, ".age")
		if !ok {
			break
		}
		filename = withoutAge
	}
	if filename == "" {
		filename = "share"
	}
	return filename + ".age"
}

func setShareContentDisposition(result *domain.ShareRenderResult, filename string) {
	if result.Headers == nil {
		result.Headers = map[string]string{}
	}
	for name := range result.Headers {
		if strings.EqualFold(name, "Content-Disposition") {
			delete(result.Headers, name)
		}
	}
	result.Headers["Content-Disposition"] = mime.FormatMediaType("inline", map[string]string{"filename": filename})
}
