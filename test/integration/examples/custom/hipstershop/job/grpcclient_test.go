// +build job

package job

import (
	"context"
	"testing"
	"time"

	hipstershop "github.com/skupperproject/skupper/test/integration/examples/custom/hipstershop/client"
	"google.golang.org/grpc"
)

const (
	timeout = 30 * time.Second
)

func TestCartService(t *testing.T) {
	// Creating the channel
	//url := "127.0.0.1:7070"
	url := "cartservice:7070"
	conn := connect(t, url)
	defer conn.Close()

	// Create a context
	ctx, cn := context.WithTimeout(context.Background(), timeout)
	defer cn()

	// Creating the stub
	client := hipstershop.NewCartServiceClient(conn)

	// Testing AddItem procedure
	t.Run("AddItem", func(t *testing.T) {
		for _, productId := range []string{"OLJCESPC7Z", "1YMWWN1N4O"} {
			// AddItem to populate cart
			_, err := client.AddItem(ctx, &hipstershop.AddItemRequest{
				UserId: "user1",
				Item: &hipstershop.CartItem{
					ProductId: productId,
					Quantity:  2,
				},
			})
			if err != nil {
				t.Fatalf("error adding item to cart - %s", err)
			}
		}
	})

	// Testing GetCart
	t.Run("GetCart", func(t *testing.T) {
		cart, err := client.GetCart(ctx, &hipstershop.GetCartRequest{
			UserId: "user1",
		})
		if err != nil {
			t.Fatalf("error retrieving cart for user1 - %s", err)
		}
		t.Logf("Cart has %d items", len(cart.Items))
	})

	// Testing EmptyCart
	t.Run("EmptyCart", func(t *testing.T) {
		_, err := client.EmptyCart(ctx, &hipstershop.EmptyCartRequest{
			UserId: "user1",
		})
		if err != nil {
			t.Fatalf("error clearing cart - %s", err)
		}
	})
}

func TestRecommendationService(t *testing.T) {
	// Connecting
	//url := "127.0.0.1:8080"
	url := "recommendationservice:8080"
	conn := connect(t, url)
	defer conn.Close()

	// Creating context
	ctx, cn := context.WithTimeout(context.Background(), timeout)
	defer cn()

	// Creating the stub
	client := hipstershop.NewRecommendationServiceClient(conn)

	t.Run("ListRecommendations", func(t *testing.T) {
		res, err := client.ListRecommendations(ctx, &hipstershop.ListRecommendationsRequest{
			UserId: "user1",
		})
		if err != nil {
			t.Fatalf("error retrieving recommendations - %s", err)
		}
		t.Logf("number of returned products: %d", len(res.ProductIds))
	})
}

func TestProductCatalogService(t *testing.T) {
	// Connecting
	//url := "127.0.0.1:3550"
	url := "productcatalogservice:3550"
	conn := connect(t, url)
	defer conn.Close()

	// Creating context
	ctx, cn := context.WithTimeout(context.Background(), timeout)
	defer cn()

	// Creating the stub
	client := hipstershop.NewProductCatalogServiceClient(conn)

	// ListProducts
	t.Run("ListProducts", func(t *testing.T) {
	})
	products, err := client.ListProducts(ctx, &hipstershop.Empty{})
	if err != nil {
		t.Fatalf("error retrieving product list - %v", err)
	}
	t.Logf("%d products found", len(products.Products))

	// GetProduct
	t.Run("GetProduct", func(t *testing.T) {
		for _, product := range products.Products {
			t.Logf("%-10s %-30s %s $%10d.%02.0f\n", product.Id, product.Name, product.PriceUsd.CurrencyCode, product.PriceUsd.Units, float32(product.PriceUsd.Nanos/10000000))
			pId, err := client.GetProduct(ctx, &hipstershop.GetProductRequest{Id: product.Id})
			if err != nil {
				t.Fatalf("error retrieving product by id - %s", err)
			}
			t.Logf("%-10s %-30s %s", pId.Id, pId.Name, pId.Description)
		}
	})

	// SearchProducts
	t.Run("SearchProducts", func(t *testing.T) {
		for _, product := range products.Products {
			res, err := client.SearchProducts(ctx, &hipstershop.SearchProductsRequest{Query: product.Name[0:3]})
			if err != nil {
				t.Fatalf("error searching products - %s", err)
			}
			t.Logf("%d products found using [%s]", len(res.Results), product.Name[0:3])
		}
	})
}

func TestShippingService(t *testing.T) {
	// Connecting
	//url := "127.0.0.1:50051"
	url := "shippingservice:50051"
	conn := connect(t, url)
	defer conn.Close()

	// Creating context
	ctx, cn := context.WithTimeout(context.Background(), timeout)
	defer cn()

	// Creating the stub
	client := hipstershop.NewShippingServiceClient(conn)

	// GetQuote
	t.Run("GetQuote", func(t *testing.T) {
		quoteRes, err := client.GetQuote(ctx, &hipstershop.GetQuoteRequest{
			Address: &hipstershop.Address{
				ZipCode: 94203,
			},
			Items: []*hipstershop.CartItem{
				{
					ProductId: "OLJCESPC7Z",
					Quantity:  1,
				},
				{
					ProductId: "1YMWWN1N4O",
					Quantity:  2,
				},
			},
		})

		if err != nil {
			t.Fatalf("error getting quote - %s", err)
		}
		t.Logf("%v", quoteRes.CostUsd)
	})

	// ShipOrder
	t.Run("ShipOrder", func(t *testing.T) {
		shipRes, err := client.ShipOrder(ctx, &hipstershop.ShipOrderRequest{
			Address: &hipstershop.Address{
				ZipCode: 94203,
			},
			Items: []*hipstershop.CartItem{
				{
					ProductId: "OLJCESPC7Z",
					Quantity:  1,
				},
				{
					ProductId: "1YMWWN1N4O",
					Quantity:  2,
				},
			}},
		)
		if err != nil {
			t.Fatalf("error shipping order - %s", err)
		}
		t.Logf("Tracking ID: %s", shipRes.TrackingId)
	})
}

func TestCurrencyService(t *testing.T) {
	// Connecting
	//url := "127.0.0.1:7000"
	url := "currencyservice:7000"
	conn := connect(t, url)
	defer conn.Close()

	// Creating context
	ctx, cn := context.WithTimeout(context.Background(), timeout)
	defer cn()

	// Creating the stub
	client := hipstershop.NewCurrencyServiceClient(conn)

	// GetSupportedCurrencies
	t.Run("GetSupportedCurrencies", func(t *testing.T) {
		res, err := client.GetSupportedCurrencies(ctx, &hipstershop.Empty{})
		if err != nil {
			t.Fatalf("error retrieving supported currencies - %s", err)
		}
		t.Logf("Supported currencies: %v", res.CurrencyCodes)
	})

	// Convert
	t.Run("Convert", func(t *testing.T) {
		convertRes, err := client.Convert(ctx, &hipstershop.CurrencyConversionRequest{
			From: &hipstershop.Money{
				CurrencyCode: "USD",
				Units:        1,
				Nanos:        0,
			},
			ToCode: "BRL",
		})
		if err != nil {
			t.Fatalf("error converting USD to BRL - %s", err)
		}
		t.Logf("%s %d.%d", convertRes.CurrencyCode, convertRes.Units, convertRes.Nanos/10000000)
	})
}

func TestPaymentService(t *testing.T) {
	// Connecting
	//url := "127.0.0.1:50051"
	url := "paymentservice:50051"
	conn := connect(t, url)
	defer conn.Close()

	// Creating context
	ctx, cn := context.WithTimeout(context.Background(), timeout)
	defer cn()

	// Creating the stub
	client := hipstershop.NewPaymentServiceClient(conn)

	// Charge
	t.Run("Charge", func(t *testing.T) {
		chargeRes, err := client.Charge(ctx, &hipstershop.ChargeRequest{
			Amount: &hipstershop.Money{
				CurrencyCode: "URL",
				Units:        100,
				Nanos:        0,
			},
			CreditCard: &hipstershop.CreditCardInfo{
				CreditCardNumber:          "5555555555554444",
				CreditCardCvv:             111,
				CreditCardExpirationYear:  2199,
				CreditCardExpirationMonth: 1,
			},
		})

		if err != nil {
			t.Fatalf("error charging credit card - %s", err)
		}

		t.Logf("Credit card has been charged. Transaction ID: %s", chargeRes.TransactionId)
	})
}

func TestEmailService(t *testing.T) {
	// Connecting
	//url := "127.0.0.1:8080" // service exposed at 5000, but container port is 8080
	url := "emailservice:5000"
	conn := connect(t, url)
	defer conn.Close()

	// Creating context
	ctx, cn := context.WithTimeout(context.Background(), timeout)
	defer cn()

	// Creating the stub
	client := hipstershop.NewEmailServiceClient(conn)

	// SendOrderConfirmation
	t.Run("SendOrderConfirmation", func(t *testing.T) {
		_, err := client.SendOrderConfirmation(ctx, &hipstershop.SendOrderConfirmationRequest{
			Email: "user1@skupper.io",
			Order: &hipstershop.OrderResult{
				OrderId:            "1234",
				ShippingTrackingId: "1111",
				ShippingCost: &hipstershop.Money{
					CurrencyCode: "USD",
					Units:        10,
					Nanos:        0,
				},
				ShippingAddress: &hipstershop.Address{
					ZipCode: 94203,
				},
				Items: []*hipstershop.OrderItem{
					&hipstershop.OrderItem{
						Item: &hipstershop.CartItem{
							ProductId: "OLJCESPC7Z",
							Quantity:  1,
						},
						Cost: &hipstershop.Money{
							CurrencyCode: "USD",
							Units:        10,
							Nanos:        0,
						},
					},
					&hipstershop.OrderItem{
						Item: &hipstershop.CartItem{
							ProductId: "1YMWWN1N4O",
							Quantity:  2,
						},
						Cost: &hipstershop.Money{
							CurrencyCode: "USD",
							Units:        20,
							Nanos:        0,
						},
					},
				},
			},
		})

		if err != nil {
			t.Fatalf("error sending order confirmation - %s", err)
		}
		t.Logf("order confirmation sent")
	})
}

func TestCheckoutService(t *testing.T) {
	// Connecting
	//url := "127.0.0.1:5050"
	url := "checkoutservice:5050"
	conn := connect(t, url)
	defer conn.Close()

	// Creating context
	ctx, cn := context.WithTimeout(context.Background(), timeout)
	defer cn()

	// Creating the stub
	client := hipstershop.NewCheckoutServiceClient(conn)

	// PlaceOrder
	t.Run("PlaceOrder", func(t *testing.T) {
		orderRes, err := client.PlaceOrder(ctx, &hipstershop.PlaceOrderRequest{
			UserId:       "user1",
			UserCurrency: "USD",
			Address: &hipstershop.Address{
				ZipCode: 94203,
			},
			Email: "user1@skupper.io",
			CreditCard: &hipstershop.CreditCardInfo{
				CreditCardNumber:          "5555555555554444",
				CreditCardCvv:             111,
				CreditCardExpirationYear:  2199,
				CreditCardExpirationMonth: 1,
			},
		})

		if err != nil {
			t.Fatalf("error placing order - %s", err)
		}

		t.Logf("order has been placed - ID = %s", orderRes.Order.OrderId)
	})
}

func TestAdService(t *testing.T) {
	// Connecting
	//url := "127.0.0.1:9555"
	url := "adservice:9555"
	conn := connect(t, url)
	defer conn.Close()

	// Creating context
	ctx, cn := context.WithTimeout(context.Background(), timeout)
	defer cn()

	// Creating the stub
	client := hipstershop.NewAdServiceClient(conn)

	// GetAds
	t.Run("GetAds", func(t *testing.T) {
		adRes, err := client.GetAds(ctx, &hipstershop.AdRequest{
			ContextKeys: []string{},
		})
		if err != nil {
			t.Fatalf("error retrieving ads - %s", err)
		}
		t.Logf("Advertises: %v", adRes.Ads)
	})
}

// connect creates a grpc connection channel or fail test if an error occurs
func connect(t *testing.T, url string) *grpc.ClientConn {
	conn, err := grpc.Dial(url, grpc.WithInsecure())
	if err != nil {
		t.Fatalf("unable to connect to %s - %s", url, err)
	}
	return conn
}
