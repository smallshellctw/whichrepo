package dev.whichrepo.ledger;

import dev.whichrepo.payments.PaymentSettled;

public final class ReconciliationJob {
    public void reconcile(PaymentSettled event) {
        // Persist balanced ledger entries for the settled payment.
    }
}
