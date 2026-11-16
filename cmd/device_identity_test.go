package cmd

import (
	"context"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Device identity configuration", func() {
	BeforeEach(func() {
		home := GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		GinkgoT().Setenv("HOMEDRIVE", filepath.VolumeName(home))
		GinkgoT().Setenv("HOMEPATH", strings.TrimPrefix(home, filepath.VolumeName(home)))
		GinkgoT().Setenv("IPATOOL_DEVICE_MAC_ADDRESS", "00:aa:bb:cc:dd:ee")
	})

	DescribeTable("resolves the explicit flag before the environment", func(setFlag bool, value, expected string) {
		command := rootCmd()
		command.SetContext(context.WithValue(context.Background(), interactiveKey, false))
		Expect(command.PersistentFlags().Set("keychain-passphrase", "test-only")).To(Succeed())
		if setFlag {
			Expect(command.PersistentFlags().Set("device-mac-address", value)).To(Succeed())
		}

		initWithCommand(command)
		address, err := dependencies.Machine.MacAddress()
		Expect(err).NotTo(HaveOccurred())
		if expected != "" {
			Expect(address).To(Equal(expected))
		} else {
			Expect(address).NotTo(Equal("00:aa:bb:cc:dd:ee"))
		}
	},
		Entry("environment default", false, "", "00:aa:bb:cc:dd:ee"),
		Entry("explicit override", true, "00:11:22:33:44:55", "00:11:22:33:44:55"),
		Entry("explicit automatic detection", true, "", ""),
	)
})
