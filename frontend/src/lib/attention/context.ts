import { createContext } from 'svelte';
import type { AttentionState } from './model.ts';

const [getAttention, setAttention] = createContext<() => AttentionState>();
export const provideAttention = setAttention;
export const useAttention = getAttention;
