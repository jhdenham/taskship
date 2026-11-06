package core

import "strings"

// blockedPrefixes are task prefixes that DefaultPrefix never suggests: swears,
// slurs, and crude abbreviations, plus their vowel-less forms, since prefixes
// are mostly consonants. Users can still type any prefix they like.
//
// The entries are ROT13-encoded to protect sensitive eyes and keep this
// codebase PG. Decode them at your own risk. To add one:
//
//	echo WORD | tr 'A-Za-z' 'N-ZA-Mn-za-m'
var blockedPrefixes = decodeBlocklist(`
	SHPX SHX SPX SPHX SHP SXA
	FUVG FUG
	PHAG PAG
	PBPX PBX QVPX QVX QPX
	CVFF CVF
	GJNG GJG GVG GVGF
	JNAX JAX
	NANY NAHF NEFR NFF OHGG
	PHZ WVMM WVM WMM
	CBEA CEA FRK FRKL FKL ZVYS GUBG AFSJ INT
	FYHG FYG JUBE JUE UBR
	OVPU OGPU
	SNT SNTF STG
	AVTT AVTN ATTN ATTE ATE AVT ATT
	XXX ANMV
	ENCR ECR
	JGS FGSH SSF TGSB XLF
	QNZA URYY PENC PEC
	OBBO CBB CBBC
	CFFL CHFL CHFF
	QLXR QLX UBZB
	FCVP FCP FCX PUAX XVXR XLX PBBA TBBX WNC JBC QNTB
	GNEQ GEQ EGEQ
`)

func decodeBlocklist(encoded string) map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(encoded) {
		m[rot13(w)] = true
	}
	return m
}

// rot13 rotates each ASCII letter 13 places; applying it twice is a no-op.
func rot13(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return 'a' + (r-'a'+13)%26
		case r >= 'A' && r <= 'Z':
			return 'A' + (r-'A'+13)%26
		}
		return r
	}, s)
}
