from shared_contracts import PaymentAttempt


def score_payment(attempt: PaymentAttempt) -> float:
    """Return a deterministic risk score used by checkout."""
    return 0.82 if attempt.country_mismatch else 0.08
