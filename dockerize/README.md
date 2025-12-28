# Docker Integration
- `.dockerignore`: the `.env` file should not be in the image/container
- `Dockerfile`: how do we build the docker image for our backend project
- `docker-compose.yml`: how we manage the deployment of multiple docker containers
- `.env` any secrets for our environment

## Notes
- The go project should listen on `0.0.0.0:8080` or some other valid port
- `gotdotenv` is not used for production. It is only used in development to easily load
`.env` variables without having to use a run script

## Docker commands without docker compose
```
docker build -t my-golang-app .
docker run -it -p 8080:8080 --rm --env-file ./.env --name my-running-app my-golang-app
```