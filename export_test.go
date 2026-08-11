package cobertura

// ParseProfileForTest exposes the internal parseProfile method for black-box testing.
func (cov *Coverage) ParseProfileForTest(profile *Profile) error {
	return cov.parseProfile(profile)
}

// CoberturaDTDDeclForTest exposes the internal DTD declaration for testing.
const CoberturaDTDDeclForTest = coberturaDTDDecl
