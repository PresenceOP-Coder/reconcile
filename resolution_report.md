# Financial Reconciliation Exception Report
**To:** Finance Operations Team  
**From:** AI Finance Controller  
**Date:** October 30, 2023  
**Subject:** Resolution Instructions for 15 Automated Reconciliation Exceptions

---

### Group 1: Amount Mismatches (VAT / Fee Discrepancies)
**Records:** `BK-20`, `GW-20`, `LD-20` (RefID 'REF-0020')

*   **What Went Wrong:** The Bank and Ledger agree on an amount of **$95.04**, but the Gateway recorded **$118.80** (a 20.00% difference). This exact 20% delta strongly indicates an unrecorded 20% VAT/tax or a processing fee that was deducted at the gateway level but not captured in the ledger or bank settlement.
*   **Actionable Next Step:** 
    1. Retrieve the transaction detail for REF-0020 from the gateway portal (e.g., Stripe/Adyen).
    2. Confirm if the 20% difference represents VAT or gateway fees.
    3. Post an adjusting journal entry to the ledger to record the tax/fee expense and align the gateway balance.

---

### Group 2: Date Drift (Settlement Delays)
**Records:** `BK-21`, `GW-21`, `LD-21` (RefID 'REF-0021')

*   **What Went Wrong:** The transaction date gap is 7 days, which exceeds our automated matching threshold of 3 days. This was caused by a delayed bank settlement over a weekend/holiday period.
*   **Actionable Next Step:** 
    1. Manually force-match these three records in the reconciliation system. 
    2. No ledger adjustments are required as all balances are correct; this is purely a timing mismatch.

---

### Group 3: Duplicate Gateway References (Double-Billing Risk)
**Records:** `BK-22`, `GW-22-1`, `GW-22-2`, `LD-22` (RefID 'REF-0022')

*   **What Went Wrong:** The gateway generated two separate records with the same Reference ID (`REF-0022`), resulting in a 50% sum variance when matching against single Bank and Ledger records.
*   **Actionable Next Step:** 
    1. Inspect the gateway dashboard for `REF-0022` to see if the customer was double-charged or if it was a system transmission error.
    2. If a double-charge occurred, initiate a refund for the duplicate transaction.
    3. If it was an API transmission error, void the duplicate gateway record (`GW-22-2`) in the sub-ledger.

---

### Group 4: Ingestion Failures & Resulting Orphans (Data Corruption)
**Records:** `GW-25`, `BK-25`, `LD-25` (RefID 'REF-0025')

*   **What Went Wrong:** The gateway record (`GW-25`) contained a corrupted date format (`INVALID_DATE_FORMAT_2023-99-99`) on Row 28 of the source file. Because the gateway record failed to ingest properly, the corresponding Bank (`BK-25`) and Ledger (`LD-25`) records could not find their counterpart.
*   **Actionable Next Step:** 
    1. Locate Row 28 in the raw Gateway CSV file. 
    2. Correct the corrupt date value to the actual transaction date.
    3. Manually re-upload the corrected gateway file or trigger a re-ingestion of Row 28 to resolve the mismatch.

---

### Group 5: Unpaired Records (Orphaned Transactions)
**Records:** `LD-23` (RefID 'REF-0023'), `GW-24` (RefID 'REF-0024')

*   **What Went Wrong:** 
    *   `LD-23`: A ledger entry exists with no corresponding bank or gateway transaction. This suggests a manual journal entry error or a cancelled invoice that was never processed.
    *   `GW-24`: A gateway transaction exists with no matching ledger entry or bank deposit. This suggests a completed transaction that failed to sync to the ERP, or funds that are still pending payout.
*   **Actionable Next Step:**
    *   **For LD-23:** Cross-reference the ledger entry creator in ERP. If it was a duplicate or cancelled invoice, reverse the ledger entry.
    *   **For GW-24:** Log into the Gateway portal, locate `REF-0024`, check if the payout has been initiated, and manually trigger the webhook to push the sale details into the ERP/Ledger.