import type { Bill, Persona } from '#lib/types.ts';

/**
 * Steward conflict rule (design/05): stewards do not mark, combine, or annotate rider
 * tags on bills their own member sponsors. They may still answer factual questions.
 */
export function stewardConflict(persona: Persona, bill: Bill): boolean {
	if (persona.role !== 'steward' || !persona.office) return false;
	const member = persona.office.replace(/^Office of /, '');
	return bill.sponsor.startsWith(member);
}
