## run/api: run the api with default settings
run/api:
	go run ./cmd/api

## run/api/production: run the api on port 3030 in production mode  
run/api/production:
	go run ./cmd/api -port=3030 -env=production

## run/api/limiter: run the api with custom rate limiter settings
run/api/limiter:
	go run ./cmd/api/ -limiter-burst=2 