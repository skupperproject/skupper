# Skupper CLI

The [Skupper](https://skupper.io/) command line enables you to:

* Create a Virtual Application Network (VAN) site
* Connect to other VAN sites
* Connect services

## Getting `skupper`

To install `skupper`, download the [latest release](https://github.com/skupperproject/skupper/releases).


## Using `skupper`

To create your first VAN site:

```
skupper init
skupper connection-token /path/to/mysecret.yaml
```

To expose a service from the first site:

```
skupper expose
```

To connect to the first site from a second site:

```
skupper connect --secret /path/to/mysecret.yaml
```

For a complete list of `skupper` commands:

```
skupper help
```

For more information see the [Skupper Documentation](https://skupper.io/docs/index.html).
