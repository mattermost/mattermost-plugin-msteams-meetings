# Include custome targets and environment variables here

E2E_DIR = e2e/playwright

$(E2E_DIR)/node_modules: $(E2E_DIR)/package.json $(E2E_DIR)/package-lock.json
	cd $(E2E_DIR) && npm ci && npx playwright install chromium
	touch $@

## Starts the disposable Mattermost server used by the E2E suite.
.PHONY: e2e-env-up
e2e-env-up:
	cd $(E2E_DIR) && docker compose up -d --wait

## Stops the E2E server and wipes its data.
.PHONY: e2e-env-down
e2e-env-down:
	cd $(E2E_DIR) && docker compose down -v

## Builds the plugin and runs the Playwright E2E suite against PW_BASE_URL (default http://localhost:8065).
.PHONY: e2e
e2e: dist $(E2E_DIR)/node_modules
	cd $(E2E_DIR) && npm run tsc && npm test

# If there's no MM_RUDDER_PLUGINS_PROD, add DEV data
RUDDER_WRITE_KEY = 1d5bMvdrfWClLxgK1FvV3s4U1tg
ifdef MM_RUDDER_PLUGINS_PROD
RUDDER_WRITE_KEY = $(MM_RUDDER_PLUGINS_PROD)
endif

GO_BUILD_FLAGS += -ldflags '-X "github.com/mattermost/mattermost/server/public/pluginapi/experimental/telemetry.rudderWriteKey=$(RUDDER_WRITE_KEY)"'
