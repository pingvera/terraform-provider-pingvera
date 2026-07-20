// Command terraform-provider-pingvera — Terraform-провайдер Pingvera.
// Чистый REST-клиент write-API hub (/api/v1/monitors, /api/v1/status-pages),
// вся бизнес-логика остаётся на сервере. Локальная разработка — через
// dev_overrides в ~/.terraformrc (см. README.md), публикация в реестр —
// не в этом репозитории (см. README.md «Публикация в Terraform Registry»).
package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/pingvera/terraform-provider-pingvera/internal/provider"
)

// version — версия сборки; при релизе подставляется через
// -ldflags "-X main.version=x.y.z" (goreleaser), при go build без флагов —
// "dev".
var version = "dev"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "запуск в режиме отладки (поддержка делегированных атачей debugger'а)")
	flag.Parse()

	opts := providerserver.ServeOpts{
		Address: "registry.terraform.io/pingvera/pingvera",
		Debug:   debug,
	}

	if err := providerserver.Serve(context.Background(), provider.New(version), opts); err != nil {
		log.Fatal(err.Error())
	}
}
