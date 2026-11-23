---- MODULE blueprint_concurrent_edits ----
\* Two (or more) people write the same blueprint. Variables -> repo:
\*   cur   = blueprints.current_version
\*   hist  = the set of blueprint_versions.version rows
\*   base  = the base_version a request read (loadBlueprintForEdit)
\*   wrote = the version a request inserted (0 = none)
\*   resp  = (blueprint.current_version, version.version) in the HTTP response
\* FIXED = FALSE models today's code: the response is read in a separate step after the commit.
\* FIXED = TRUE  models S7: the response is taken from the write's own transaction.
EXTENDS Naturals, FiniteSets
CONSTANTS Procs, MaxV, FIXED
VARIABLES cur, hist, pc, base, wrote, resp
vars == <<cur, hist, pc, base, wrote, resp>>

Init == /\ cur = 1 /\ hist = {1}
        /\ pc = [p \in Procs |-> "idle"]
        /\ base = [p \in Procs |-> 0]
        /\ wrote = [p \in Procs |-> 0]
        /\ resp = [p \in Procs |-> <<0, 0>>]

\* S2: read the current version (Get + the base_version test; stale reads are rejected in code, here a stale base just loses the CAS)
Load(p) == /\ pc[p] = "idle"
           /\ base' = [base EXCEPT ![p] = cur]
           /\ pc' = [pc EXCEPT ![p] = "loaded"]
           /\ UNCHANGED <<cur, hist, wrote, resp>>

\* S5: AppendVersion = one transaction: UPDATE ... WHERE current_version = base, then INSERT the version row
Commit(p) == /\ pc[p] = "loaded"
             /\ IF cur = base[p] /\ cur < MaxV
                THEN /\ cur' = cur + 1
                     /\ hist' = hist \cup {cur + 1}
                     /\ wrote' = [wrote EXCEPT ![p] = cur + 1]
                     /\ resp' = [resp EXCEPT ![p] = IF FIXED THEN <<cur + 1, cur + 1>> ELSE resp[p]]
                     /\ pc' = [pc EXCEPT ![p] = IF FIXED THEN "done" ELSE "written"]
                ELSE /\ pc' = [pc EXCEPT ![p] = "conflict"]   \* 409, nothing written
                     /\ UNCHANGED <<cur, hist, wrote, resp>>
             /\ UNCHANGED base

\* S6 today: Get(blueprint) and GetVersion(base+1) after the commit, outside the transaction
Respond(p) == /\ pc[p] = "written"
              /\ resp' = [resp EXCEPT ![p] = <<cur, wrote[p]>>]
              /\ pc' = [pc EXCEPT ![p] = "done"]
              /\ UNCHANGED <<cur, hist, base, wrote>>

\* failure action: Agent/Engine error, client left, process crash after the load and before the write
Crash(p) == /\ pc[p] = "loaded"
            /\ pc' = [pc EXCEPT ![p] = "failed"]
            /\ UNCHANGED <<cur, hist, base, wrote, resp>>

Next == \E p \in Procs : Load(p) \/ Commit(p) \/ Respond(p) \/ Crash(p)
Spec == Init /\ [][Next]_vars

HistContiguous == hist = 1..cur                                    \* no gap, no lost version row
OneWriterPerVersion == \A p, q \in Procs : (p # q /\ wrote[p] > 0) => wrote[p] # wrote[q]
WriteFromBase == \A p \in Procs : wrote[p] > 0 => wrote[p] = base[p] + 1   \* no write on a stale base
RespConsistent == \A p \in Procs : pc[p] = "done" => resp[p][1] = resp[p][2]
====
