//go:build wireinject
// +build wireinject

package main

import (
	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"

	"github.com/makesalekz/products/internal/biz"
	"github.com/makesalekz/products/internal/conf"
	"github.com/makesalekz/products/internal/data"
	"github.com/makesalekz/products/internal/server"
	"github.com/makesalekz/products/internal/service"
)

func wireApp(*conf.Bootstrap, log.Logger) (*kratos.App, func(), error) {
	panic(wire.Build(server.ProviderSet, data.ProviderSet, biz.ProviderSet, service.ProviderSet, newApp))
}
