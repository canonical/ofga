# Contributing

## Testing

### Unit Tests

Unit tests use mocked HTTP responses and don't require any external services.

```bash
make test
```

### Integration Tests

Integration tests run against a real OpenFGA instance with PostgreSQL.

```bash
docker compose up -d 
make test-integration
```

The integration tests in `integrationtesting/` create a temporary store, load the authorization model from `test-model.json`, run tests, and clean up.
