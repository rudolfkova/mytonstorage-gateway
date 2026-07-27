package htmlTemplates

import (
	"html/template"
	"path/filepath"
	"slices"
	"strings"

	"mytonstorage-gateway/pkg/models/private"
	"mytonstorage-gateway/pkg/utils"
)

const APIBase = "/api/v1/gateway"

var (
	imageFormats = []string{"jpg", "jpeg", "png", "gif", "webp", "svg", "bmp", "tiff", "ico", "avif"}
	videoFormats = []string{"mp4", "mkv", "webm", "avi", "mov", "flv", "wmv"}
	audioFormats = []string{"mp3", "wav", "ogg", "flac", "aac", "m4a"}
	textFormats  = []string{"txt", "md", "csv", "json", "xml", "html", "htm", "xhtml", "css", "js", "ts", "py", "go", "java", "c", "cpp", "h", "php", "rb", "sh"}

	htmlFormats = []string{"html", "htm", "xhtml"}
)

type htmlTemplates struct {
	templates *template.Template
}

type Templates interface {
	ContentType(filename string) ContentType
	HtmlFilesListWithTemplate(f private.FolderInfo, path string) (string, error)
	HtmlLoadingPage() (string, error)
}

// ContentType returns the appropriate Content-Type header and value based on the file extension.
// If the file type is not recognized, it returns a Content-Disposition header to prompt download.
func (t *htmlTemplates) ContentType(filename string) ContentType {
	ext := strings.ToLower(filepath.Ext(filename))
	ext = strings.TrimPrefix(ext, ".")
	if index := slices.IndexFunc(imageFormats, func(s string) bool { return strings.HasSuffix(ext, s) }); index != -1 {
		return ContentType{
			Header:     "Content-Type",
			Value:      "image/" + imageFormats[index],
			IsDownload: false,
			IsHtml:     false,
		}
	} else if index := slices.IndexFunc(videoFormats, func(s string) bool { return strings.HasSuffix(ext, s) }); index != -1 {
		return ContentType{
			Header:     "Content-Type",
			Value:      "video/" + videoFormats[index],
			IsDownload: false,
			IsHtml:     false,
		}
	} else if index := slices.IndexFunc(audioFormats, func(s string) bool { return strings.HasSuffix(ext, s) }); index != -1 {
		return ContentType{
			Header:     "Content-Type",
			Value:      "audio/" + audioFormats[index],
			IsDownload: false,
			IsHtml:     false,
		}
	} else if index := slices.IndexFunc(textFormats, func(s string) bool { return strings.HasSuffix(ext, s) }); index != -1 {
		var mimeType string
		switch textFormats[index] {
		case "txt", "md":
			mimeType = "text/plain; charset=utf-8"
		case "html", "htm", "xhtml":
			mimeType = "text/html; charset=utf-8"
		case "css":
			mimeType = "text/css; charset=utf-8"
		case "js":
			mimeType = "application/javascript; charset=utf-8"
		case "json":
			mimeType = "application/json; charset=utf-8"
		case "xml":
			mimeType = "application/xml; charset=utf-8"
		case "csv":
			mimeType = "text/csv; charset=utf-8"
		default:
			mimeType = "text/plain; charset=utf-8"
		}

		return ContentType{
			Header:     "Content-Type",
			Value:      mimeType,
			IsDownload: false,
			IsHtml:     slices.Contains(htmlFormats, textFormats[index]),
		}
	}

	return ContentType{
		Header:     "Content-Disposition",
		Value:      `attachment; filename="` + filename + `"`,
		IsDownload: true,
		IsHtml:     false,
	}
}

func (t *htmlTemplates) HtmlFilesListWithTemplate(f private.FolderInfo, path string) (string, error) {
	f.BagID = strings.ToUpper(f.BagID)

	data := TemplateData{
		Title:       filepath.Join(f.BagID, path),
		FullPath:    filepath.Join(f.BagID, path),
		TotalSize:   utils.FormatSize(f.TotalSize),
		Description: f.Description,
		PeersCount:  f.PeersCount,
		FileCount:   f.FilesCount,
		Files:       make([]FileData, 0, len(f.Files)),
	}

	if path != "" {
		parentPath := filepath.Dir(path)
		var parentHref string
		if parentPath == "." || parentPath == "" {
			parentHref = filepath.Join(APIBase, f.BagID)
		} else {
			parentHref = filepath.Join(APIBase, f.BagID, parentPath)
		}
		data.ParentDir = &ParentDirData{Href: parentHref}
	}

	for _, file := range f.Files {
		icon := "📄"
		ext := filepath.Ext(file.Name)
		if file.IsFolder {
			icon = "📁"
		} else if slices.Contains(imageFormats, ext) {
			icon = "🖼️"
		} else if slices.Contains(videoFormats, ext) {
			icon = "🎥"
		} else if slices.Contains(audioFormats, ext) {
			icon = "🎵"
		} else if slices.Contains(textFormats, ext) {
			icon = "📝"
		}

		var href string
		if path == "" {
			href = filepath.Join(APIBase, f.BagID, file.Name)
		} else {
			href = filepath.Join(APIBase, f.BagID, path, file.Name)
		}

		fileSize := utils.FormatSize(file.Size)
		if file.Size == 0 {
			fileSize = "folder"
		}

		data.Files = append(data.Files, FileData{
			Name:          file.Name,
			Href:          href,
			Icon:          icon,
			FormattedSize: fileSize,
		})
	}

	return t.renderTemplate("file_list.html", &data)
}

func (t *htmlTemplates) HtmlLoadingPage() (string, error) {
	return t.renderTemplate("loading.html", nil)
}

func (t *htmlTemplates) renderTemplate(templateName string, data any) (string, error) {
	var buf strings.Builder
	err := t.templates.ExecuteTemplate(&buf, templateName, data)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func New(templatesPath string) (Templates, error) {
	templates, err := template.ParseGlob(filepath.Join(templatesPath, "*.html"))
	if err != nil {
		return nil, err
	}

	return &htmlTemplates{
		templates: templates,
	}, nil
}
