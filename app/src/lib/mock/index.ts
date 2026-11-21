import type { Bill, BillVersion, Section } from '#lib/types.ts';
import { bills } from './bills';
import { issues } from './issues';

export { bills } from './bills';
export { issues } from './issues';
export { personas, personaById } from './personas';
export { seedThreads, seedSuggestions, seedSignals, BRIEF_MINIMUM, SMALL_COUNT } from './seed';

export const billById = (id: string): Bill | undefined => bills.find((b) => b.id === id);
export const issueById = (id: string) => issues.find((i) => i.id === id);

export const latestVersion = (bill: Bill): BillVersion => bill.versions[bill.versions.length - 1];

export const versionOf = (bill: Bill, n: number): BillVersion | undefined =>
	bill.versions.find((v) => v.n === n);

export const sectionOf = (version: BillVersion, sectionId: string): Section | undefined =>
	version.sections.find((s) => s.id === sectionId);
