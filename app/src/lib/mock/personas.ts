import type { Persona } from '#lib/types.ts';

// Mock personas. Members and staff are placeholders, never real people.
export const personas: Persona[] = [
	{
		id: 'reader',
		name: 'Reader',
		role: 'reader',
		blurb: 'No account. Read and share only.'
	},
	{
		id: 'reviewer',
		name: 'Dana R.',
		role: 'reviewer',
		blurb: 'Comment, suggest, support, oppose.'
	},
	{
		id: 'steward-a',
		name: '[Steward A]',
		role: 'steward',
		office: 'Office of Rep. [Member A]',
		party: 'A',
		blurb: "Party A office. Sponsor's party, so the conflict rule applies on H.R. 4127."
	},
	{
		id: 'steward-b',
		name: '[Steward B]',
		role: 'steward',
		office: 'Office of Sen. [Member B]',
		party: 'B',
		blurb: 'Party B office. Can mark suggestions well formed.'
	},
	{
		id: 'moderator',
		name: '[Moderator]',
		role: 'moderator',
		blurb: 'Sees flags. Removal flow is a placeholder.'
	},
	{
		id: 'office',
		name: '[Staffer]',
		role: 'office',
		office: 'Office of Rep. [Member C]',
		blurb: 'Read briefs and respond to items. Cannot signal.'
	}
];

export const personaById = (id: string) => personas.find((p) => p.id === id) ?? personas[0];
