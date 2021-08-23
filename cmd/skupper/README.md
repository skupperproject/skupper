# Skupper CLI

The [Skupper](https://skupper.io/) command line enables you to:

* Create a Virtual Application Network (VAN) site
* Connect to other VAN sites
* Connect services

## Getting `skupper`

To install `skupper`, use the instructions in the [Getting Started](https://skupper.io/start/).


## Using `skupper`

To create your first VAN site:

```
skupper init
```

You can later delete that site:

```
skupper delete
```

To expose a service from the first site:

```
skupper expose
```

To connect to this site from another site, you need to create an exchange tokens, for example:

```
skupper token create /path/to/mysecret.yaml
```

This command writes a token in the specified path, you can use that token from a second site by entering:

```
skupper link create /path/to/mysecret.yaml
```

After waiting some time, check that the connection is working:

```
skupper status
```

The status of the link can be checked as well:

```
skupper link status
```

This is a simple example, many connection options are available.
For a complete list of `skupper` commands:

```
skupper help
```

For more information see the [Skupper Documentation](https://skupper.io/docs/index.html).
