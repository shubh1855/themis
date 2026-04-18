package tools

import (
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/models"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/registry"
)

func registryBase(ctx Context) *registry.BaseClient {
	return registry.NewBaseClient(ctx.Deps.HTTP, ctx.Deps.Cache)
}

func HandleNPMSearch(ctx Context) models.ToolResponse {
	query := models.ArgString(ctx.Req.Args, "query")
	if query == "" {
		return models.ErrorResponse("npm_search: missing 'query' argument")
	}
	limit := models.ArgInt(ctx.Req.Args, "limit", 10)

	npm := registry.NewNPM(registryBase(ctx))
	result, err := npm.Search(query, limit)
	if err != nil {
		return models.ErrorResponsef("npm_search: %v", err)
	}
	return models.SuccessResponse(result)
}

func HandleNPMLookup(ctx Context) models.ToolResponse {
	name := models.ArgString(ctx.Req.Args, "name")
	if name == "" {
		return models.ErrorResponse("npm_lookup: missing 'name' argument")
	}

	npm := registry.NewNPM(registryBase(ctx))
	info, err := npm.Lookup(name)
	if err != nil {
		return models.ErrorResponsef("npm_lookup: %v", err)
	}
	return models.SuccessResponse(info)
}

func HandlePipSearch(ctx Context) models.ToolResponse {
	query := models.ArgString(ctx.Req.Args, "query")
	if query == "" {
		return models.ErrorResponse("pip_search: missing 'query' argument")
	}
	limit := models.ArgInt(ctx.Req.Args, "limit", 10)

	pip := registry.NewPip(registryBase(ctx))
	result, err := pip.Search(query, limit)
	if err != nil {
		return models.ErrorResponsef("pip_search: %v", err)
	}
	return models.SuccessResponse(result)
}

func HandlePipLookup(ctx Context) models.ToolResponse {
	name := models.ArgString(ctx.Req.Args, "name")
	if name == "" {
		return models.ErrorResponse("pip_lookup: missing 'name' argument")
	}

	pip := registry.NewPip(registryBase(ctx))
	info, err := pip.Lookup(name)
	if err != nil {
		return models.ErrorResponsef("pip_lookup: %v", err)
	}
	return models.SuccessResponse(info)
}

func HandleCargoSearch(ctx Context) models.ToolResponse {
	query := models.ArgString(ctx.Req.Args, "query")
	if query == "" {
		return models.ErrorResponse("cargo_search: missing 'query' argument")
	}
	limit := models.ArgInt(ctx.Req.Args, "limit", 10)

	cargo := registry.NewCargo(registryBase(ctx))
	result, err := cargo.Search(query, limit)
	if err != nil {
		return models.ErrorResponsef("cargo_search: %v", err)
	}
	return models.SuccessResponse(result)
}

func HandleCrateLookup(ctx Context) models.ToolResponse {
	name := models.ArgString(ctx.Req.Args, "name")
	if name == "" {
		return models.ErrorResponse("crate_lookup: missing 'name' argument")
	}

	cargo := registry.NewCargo(registryBase(ctx))
	info, err := cargo.Lookup(name)
	if err != nil {
		return models.ErrorResponsef("crate_lookup: %v", err)
	}
	return models.SuccessResponse(info)
}

func HandleGoSearch(ctx Context) models.ToolResponse {
	query := models.ArgString(ctx.Req.Args, "query")
	if query == "" {
		return models.ErrorResponse("go_search: missing 'query' argument")
	}
	limit := models.ArgInt(ctx.Req.Args, "limit", 10)

	golang := registry.NewGoLang(registryBase(ctx))
	result, err := golang.Search(query, limit)
	if err != nil {
		return models.ErrorResponsef("go_search: %v", err)
	}
	return models.SuccessResponse(result)
}

func HandleGoLookup(ctx Context) models.ToolResponse {
	name := models.ArgString(ctx.Req.Args, "name")
	if name == "" {
		return models.ErrorResponse("go_lookup: missing 'name' argument")
	}

	golang := registry.NewGoLang(registryBase(ctx))
	info, err := golang.Lookup(name)
	if err != nil {
		return models.ErrorResponsef("go_lookup: %v", err)
	}
	return models.SuccessResponse(info)
}
