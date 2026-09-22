<?php

namespace Commerce\Catalog;

use Commerce\Shared\Money;

final class PricingService
{
    public function price(string $sku): Money
    {
        return new Money(1299, 'USD');
    }
}
