using Company.Payments.Contracts;

namespace Billing.Api;

public sealed class InvoicesController
{
    public Invoice GetInvoice(string id) => new(id);
}
