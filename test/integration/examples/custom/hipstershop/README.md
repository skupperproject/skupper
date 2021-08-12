# Hipster shop demo integration test

This test runs the [Hipster shop demo](https://github.com/skupperproject/skupper-example-grpc) against
one or three different namespaces or clusters.

It also runs gRPC client tests as Kubernetes jobs, to validate all deployed services against all
participant namespaces or clusters.

The job is self-sufficient to validate the results, therefore the test suite just expects that all jobs
complete successfully.

## Client jobs

All clients are written within a unique container image. An environment variable,
named CLIENT must be defined when running the job to determine the client code
to execute.

The following sections contain the possible values for the CLIENT variable.

### CartService

* Add a few items to the cart
* Get the cart
* Cleans the cart

### RecommendationService

* Get list of recommendations

### ProductCatalogService

* List all products, then for each:
  * Get product by id (GetProduct)
  * Search by name (SearchProducts)

### ShippingService

* Call GetQuote
* Call ShipOrder using static list of products

### CurrencyService

* Get supported currencies
* Convert 1 USD to BRL

### PaymentService

* Calls Charge method using a sample credit card

### EmailService

* Calls Send Order Confirmation using sample data

### CheckoutService

* Calls Place Order 

### AdService

* Get suggested advertises