-- TDR 024 removes the bootstrap mechanism entirely (docs/tdr/024_drop_web_auth_gate_design.md)
-- — opusflow no longer gates anything, so there's nothing left to
-- distinguish "fresh install" from "bootstrapped, then every token
-- deleted." auth_bootstrap only ever existed to make that distinction.
DROP TABLE auth_bootstrap;
