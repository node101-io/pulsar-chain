fn main(
    session_public_key: pub Field,
    expires_at_height: pub Field,
    identity: pub Field,
    account_address: pub Field,
    witness: Field,
) {
    assert(session_public_key != 0);
    assert(expires_at_height != 0);
    assert(identity != 0);
    assert(account_address != 0);
    assert(witness != 0);
}
