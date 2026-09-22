module example.com/commerce/webhook-worker

go 1.24

require example.com/commerce/payments-api v0.0.0

replace example.com/commerce/payments-api => ../../services/payments-api
