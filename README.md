# ofga

ofga is a wrapper library for conveniently interacting with OpenFGA instances.

[OpenFGA](https://openfga.dev/) is an open-source Fine-Grained Authorization
(FGA) solution that provides a framework and set of tools for implementing
fine-grained access control and permission management in applications.

This Go library builds upon the default
[OpenFGA client](https://github.com/openfga/go-sdk) by providing a more
convenient and streamlined interface. It simplifies common interactions with
OpenFGA instances, offering an alternative API that implements a commonly-used
set of opinionated operations.

## Why ofga?

- **Convenience methods**: ofga provides a set of high-level convenience methods
that abstract away the complexities of working directly with the OpenFGA client.
This saves developers time and effort when performing common tasks.
- **Structured representation**: ofga introduces a well-defined structure for
representing relationship tuples within the OpenFGA system. This structured
approach simplifies the management of relationship tuples and their associated
system entities within your application.
- **Enhanced response format**: The library transforms certain responses
returned by the underlying client, allowing for easier data access and 
manipulation. One example is the Expand API, where the underlying client returns
a tree-like response, while the library recursively expands this tree (upto the
specified depth) to provide the actual set of users/usersets that possess the
relevant permissions.

## Quickstart

1. Install the library using the following command:
    ```shell
        go get -u github.com/canonical/ofga
    ```

2. Import the library in your code:
    ```go
        import "github.com/canonical/ofga"
    ```
   
3. Create a new ofga client and handle any errors:
    ```go
    ctx = context.Background()
    
   // Create a new ofga client
    client, err := ofga.NewClient(ctx, ofga.OpenFGAParams{
        Scheme:      os.Getenv("OPENFGA_API_SCHEME"),    // defaults to `https` if not specified.
        Host:        os.Getenv("OPENFGA_API_HOST"),
        Port:        os.Getenv("OPENFGA_API_PORT"),
        Token:       os.Getenv("SECRET_TOKEN"),           // Optional, based on the OpenFGA instance configuration.
        StoreID:     os.Getenv("OPENFGA_STORE_ID"),      // Required only when connecting to a pre-existing store.
        AuthModelID: os.Getenv("OPENFGA_AUTH_MODEL_ID"),  // Required only when connecting to a pre-existing auth model.
    })
    if err != nil {
        // Handle error
    }
    ```
4. Use the client to interact with OpenFGA instances based on your requirements.
For example:
    ```go
    err = client.AddRelation(ctx, ofga.Tuple{
        Object:   &ofga.Entity{Kind: "user", ID: "123"},
        Relation: "editor",
        Target:   &ofga.Entity{Kind: "document", ID: "ABC"},
    })
    if err != nil {
        // Handle error
    }
    ```
5. Use the client to check for relations:
    ```go
    allowed, err = client.CheckRelation(ctx, ofga.Tuple{
        Object:   &ofga.Entity{Kind: "user", ID: "123"},
        Relation: "viewer",
        Target:   &ofga.Entity{Kind: "document", ID: "ABC"},
    })
    if err != nil {
        // Handle error
    }
    if !allowed {
        // Permission denied
    }
    ... // Perform action
    ```

## Documentation

The documentation for this package can be found on
[pkg.go.dev](https://pkg.go.dev/github.com/canonical/ofga).


## Contributing

If you encounter any issues or have suggestions for improvements, please open
an issue on the [GitHub repository](https://github.com/canonical/ofga).


## Authors

[Canonical Commercial Systems Team](mailto:jaas-dev@lists.launchpad.net)
