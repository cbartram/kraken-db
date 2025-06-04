# Kraken DB

This repository defines the MySQL database and Minio deployment manifests for [Kraken Plugins](https://kraken-plugins.duckdns.org) and 
also includes scripts required for releasing new plugins, managing beta plugins, and handling plugin sales as they relate to the database.

## Getting Started

You will need MySQL installed locally and running (preferably in a docker container). You will need the following environment variables in a `.env` file:

```shell
DEV_PASSWORD=<pass>
PROD_PASSWORD=<pass>
PROD_HOST=<yourhost.duckdns.com>
```

## Running

To run and use this tool:

```shell
# Build the tool
export GOOS=linux
export GOARCH=amd64

go build -o migrate main.go

# Run migration for Dev
./migrate -db-name=kraken -db-user=kraken -db-password=$DEV_PASSWORD -db-port 3306

# Run migration for Prod
./migrate -db-name=kraken -db-user=kraken -db-host=$PROD_HOST -db-password=$PROD_PASSWORD -db-port 30306

# Dry run to see what would change
./migrate -db-name=kraken -db-user=kraken -dry-run
```

# Management

The following sections describe how you can manage and configure Kraken plugins elements like:
- Plugin Sales
- Beta plugins
- Adding plugins

## Revoking Plugin Access & Beta Plugins

Currently, there is no automated process for handling beta plugins. They are purchased with 0 tokens the same as normal plugins. When it comes time
to release it live you will need to run a script like this:

```mysql
-- Define the plugin name you want to update
SET @plugin_name = 'some_plugin_name';

-- 1. Revoke all user access to the plugin by deleting from the `plugins` table
DELETE FROM plugins
WHERE name = @plugin_name;

-- 2. Update `is_in_beta` to false in `plugin_metadata`
UPDATE plugin_metadata
SET is_in_beta = FALSE
WHERE name = @plugin_name;

-- 3. Update the plugin pricing in `plugin_metadata_price_details`
UPDATE plugin_metadata_price_details pd
    JOIN plugin_metadata pm ON pd.plugin_metadata_id = pm.id
SET pd.month = 1000,
    pd.three_month = 2700,
    pd.year = 10000
WHERE pm.name = @plugin_name;
```

## Running the tests

No tests yet.

## Deployment

This is designed to be deployed on a kubernetes cluster through Helm. Docker images are already built for both minio and 
MySQL so to deploy them to a cluster run:

```shell
helm install minio ./manifets/minio -f ./manifets/minio/values.yaml
helm install kraken-db ./manifets/kraken-db -f ./manifets/kraken-db/values.yaml
```

## Built With

- [GoLang](https://go.dev/doc/install) - Programming Language

## Contributing

Please read [CONTRIBUTING.md](CONTRIBUTING.md) for details on our code
of conduct, and the process for submitting pull requests to us.

## Versioning

We use [Semantic Versioning](http://semver.org/) for versioning. For the versions
available, see the [tags on this
repository](https://github.com/cbartram/kraken-loader-plugin/tags).

## Authors

- **C. Bartram** - *Initial Project implementation* - [RuneWraith](https://github.com/cbartram)

See also the list of [contributors](https://github.com/PurpleBooth/a-good-readme-template/contributors)who participated in this project.

## License

This project is licensed under the [CC0 1.0 Universal](LICENSE.md) Creative Commons License - see the [LICENSE.md](LICENSE.md) file for
details.

## Acknowledgments

- RuneLite for making an incredible piece of software and API.