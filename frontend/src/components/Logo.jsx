/* The mark: a serif N inside a hairline ring, set like a seal. Quiet on purpose. */
export default function Logo({ size = 32 }) {
  return (
    <svg width={size} height={size} viewBox="0 0 32 32" aria-hidden="true">
      <circle cx="16" cy="16" r="15.25" fill="none" stroke="currentColor" strokeWidth="0.75" opacity="0.5" />
      <text x="16" y="21.6" textAnchor="middle" fontFamily="Spectral, 'Iowan Old Style', Georgia, serif" fontSize="16.5" fontWeight="300" fill="currentColor">N</text>
    </svg>
  );
}
