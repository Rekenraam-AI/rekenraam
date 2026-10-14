import type { TransactionResponse } from '#lib/api/transactions.ts';

/** Register rows carry a transaction summary, not all journal entries. */
export type RowActionTransaction = Pick<TransactionResponse, 'id' | 'status' | 'needs_review' | 'transaction_kind'>;
export type TransactionRowAction = 'view' | 'edit' | 'correct' | 'approve' | 'unvoid';
export type TransactionRowActionHandler = (action: TransactionRowAction, transactionID: number) => void;
