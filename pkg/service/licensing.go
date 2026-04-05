package service

import (
	"github.com/mujhtech/dagryn/pkg/entitlement"
	"github.com/mujhtech/dagryn/pkg/licensing"
	"github.com/rs/zerolog/log"
)

func NewLicensingService(licenseKey string, entitlement entitlement.Checker) *licensing.FeatureGate {
	if entitlement != nil {
		return nil // If an external entitlement checker is provided, we don't use a license-based FeatureGate
	}

	// If no key in config/env, check the stored license (written by dashboard or CLI activation).
	if licenseKey == "" {
		if stored, err := licensing.LoadStoredLicense(); err == nil && stored.Key != "" {
			licenseKey = stored.Key
			log.Info().Msg("Loaded license key from stored activation")
		}
	}

	if licenseKey == "" {
		log.Info().Msg("No license key configured -- running as Community edition")
		return licensing.NewFeatureGate(nil, log.Logger)
	}

	var featureGate *licensing.FeatureGate
	keys, err := licensing.ParsePublicKeys()
	if err != nil {
		log.Warn().Err(err).Msg("Invalid license keyring -- running as Community edition")
		featureGate = licensing.NewFeatureGate(nil, log.Logger)
	} else if len(keys) == 0 {
		log.Warn().Msg("No license public keys embedded in binary -- running as Community edition")
		featureGate = licensing.NewFeatureGate(nil, log.Logger)
	} else {
		validator := licensing.NewValidator(keys)
		claims, err := validator.Validate(licenseKey)
		if err != nil {
			log.Warn().Err(err).Msg("Invalid license key -- running as Community edition")
			featureGate = licensing.NewFeatureGate(nil, log.Logger)
		} else {
			featureGate = licensing.NewFeatureGate(claims, log.Logger)
			log.Info().
				Str("edition", string(claims.Edition)).
				Str("customer", claims.Subject).
				Int("seats", claims.Seats).
				Int("days_remaining", claims.DaysUntilExpiry()).
				Msg("License validated")

			if featureGate.IsExpiring() {
				log.Warn().
					Int("days_remaining", claims.DaysUntilExpiry()).
					Msg("License expiring soon -- please renew")
			}
			if featureGate.InGracePeriod() {
				log.Warn().Msg("License expired -- running in grace period, features will be disabled soon")
			}
		}
	}

	return featureGate
}
