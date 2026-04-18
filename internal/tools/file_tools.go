package tools

import (
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/files"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/models"
)

func fileMgr(ctx Context) *files.Manager {
	return files.NewManager(ctx.Deps.RootDir)
}

// HandleCreateFile creates a new file (fails if exists).
func HandleCreateFile(ctx Context) models.ToolResponse {
	path := models.ArgString(ctx.Req.Args, "path")
	content := models.ArgString(ctx.Req.Args, "content")
	if path == "" {
		return models.ErrorResponse("create_file: missing 'path' argument")
	}

	if err := fileMgr(ctx).CreateFile(path, content); err != nil {
		return models.ErrorResponsef("create_file: %v", err)
	}
	return models.SuccessResponse("created " + path)
}

// HandleWriteFile writes content to a file (creates or overwrites).
func HandleWriteFile(ctx Context) models.ToolResponse {
	path := models.ArgString(ctx.Req.Args, "path")
	content := models.ArgString(ctx.Req.Args, "content")
	if path == "" {
		return models.ErrorResponse("write_file: missing 'path' argument")
	}

	if err := fileMgr(ctx).WriteFile(path, content); err != nil {
		return models.ErrorResponsef("write_file: %v", err)
	}
	return models.SuccessResponse("wrote " + path)
}

// HandleAppendFile appends content to a file.
func HandleAppendFile(ctx Context) models.ToolResponse {
	path := models.ArgString(ctx.Req.Args, "path")
	content := models.ArgString(ctx.Req.Args, "content")
	if path == "" {
		return models.ErrorResponse("append_file: missing 'path' argument")
	}

	if err := fileMgr(ctx).AppendFile(path, content); err != nil {
		return models.ErrorResponsef("append_file: %v", err)
	}
	return models.SuccessResponse("appended to " + path)
}

// HandleReadFile reads the contents of a file.
func HandleReadFile(ctx Context) models.ToolResponse {
	path := models.ArgString(ctx.Req.Args, "path")
	if path == "" {
		return models.ErrorResponse("read_file: missing 'path' argument")
	}

	content, err := fileMgr(ctx).ReadFile(path)
	if err != nil {
		return models.ErrorResponsef("read_file: %v", err)
	}
	return models.SuccessResponse(content)
}

// HandleEditFile performs a search-and-replace edit on a file.
func HandleEditFile(ctx Context) models.ToolResponse {
	path := models.ArgString(ctx.Req.Args, "path")
	oldStr := models.ArgString(ctx.Req.Args, "old_string")
	newStr := models.ArgString(ctx.Req.Args, "new_string")
	if path == "" {
		return models.ErrorResponse("edit_file: missing 'path' argument")
	}
	if oldStr == "" {
		return models.ErrorResponse("edit_file: missing 'old_string' argument")
	}

	if err := fileMgr(ctx).EditFile(path, oldStr, newStr); err != nil {
		return models.ErrorResponsef("edit_file: %v", err)
	}
	return models.SuccessResponse("edited " + path)
}

// HandleMkdir creates a directory (and parents).
func HandleMkdir(ctx Context) models.ToolResponse {
	path := models.ArgString(ctx.Req.Args, "path")
	if path == "" {
		return models.ErrorResponse("mkdir: missing 'path' argument")
	}

	if err := fileMgr(ctx).Mkdir(path); err != nil {
		return models.ErrorResponsef("mkdir: %v", err)
	}
	return models.SuccessResponse("created directory " + path)
}

// HandleDeleteFile deletes a file.
func HandleDeleteFile(ctx Context) models.ToolResponse {
	path := models.ArgString(ctx.Req.Args, "path")
	if path == "" {
		return models.ErrorResponse("delete_file: missing 'path' argument")
	}

	if err := fileMgr(ctx).DeleteFile(path); err != nil {
		return models.ErrorResponsef("delete_file: %v", err)
	}
	return models.SuccessResponse("deleted " + path)
}

// HandleMoveFile moves a file.
func HandleMoveFile(ctx Context) models.ToolResponse {
	src := models.ArgString(ctx.Req.Args, "source")
	dst := models.ArgString(ctx.Req.Args, "destination")
	if src == "" || dst == "" {
		return models.ErrorResponse("move_file: missing 'source' or 'destination' argument")
	}

	if err := fileMgr(ctx).MoveFile(src, dst); err != nil {
		return models.ErrorResponsef("move_file: %v", err)
	}
	return models.SuccessResponse("moved " + src + " → " + dst)
}

// HandleCopyFile copies a file.
func HandleCopyFile(ctx Context) models.ToolResponse {
	src := models.ArgString(ctx.Req.Args, "source")
	dst := models.ArgString(ctx.Req.Args, "destination")
	if src == "" || dst == "" {
		return models.ErrorResponse("copy_file: missing 'source' or 'destination' argument")
	}

	if err := fileMgr(ctx).CopyFile(src, dst); err != nil {
		return models.ErrorResponsef("copy_file: %v", err)
	}
	return models.SuccessResponse("copied " + src + " → " + dst)
}

// HandleListDir lists directory contents.
func HandleListDir(ctx Context) models.ToolResponse {
	path := models.ArgString(ctx.Req.Args, "path")
	if path == "" {
		path = "."
	}

	entries, err := fileMgr(ctx).ListDir(path)
	if err != nil {
		return models.ErrorResponsef("list_dir: %v", err)
	}
	return models.SuccessResponse(entries)
}

// HandleTree shows a directory tree.
func HandleTree(ctx Context) models.ToolResponse {
	path := models.ArgString(ctx.Req.Args, "path")
	if path == "" {
		path = "."
	}
	depth := models.ArgInt(ctx.Req.Args, "depth", 3)

	tree, err := fileMgr(ctx).Tree(path, depth)
	if err != nil {
		return models.ErrorResponsef("tree: %v", err)
	}
	return models.SuccessResponse(tree)
}

// HandleGlob searches for files matching a glob pattern.
func HandleGlob(ctx Context) models.ToolResponse {
	pattern := models.ArgString(ctx.Req.Args, "pattern")
	if pattern == "" {
		return models.ErrorResponse("glob_search: missing 'pattern' argument")
	}

	matches, err := fileMgr(ctx).Glob(pattern)
	if err != nil {
		return models.ErrorResponsef("glob_search: %v", err)
	}
	return models.SuccessResponse(matches)
}
