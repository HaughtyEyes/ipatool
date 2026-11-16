package machine

import (
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Configured device identity", func() {
	It("uses a normalized fixed identity without enumerating container interfaces", func() {
		sut := New(Args{DeviceMACAddress: " 00-AA-BB-CC-DD-EE "}).(*machine)
		sut.interfaces = func() ([]net.Interface, error) {
			Fail("configured identity must not enumerate interfaces")

			return nil, nil
		}

		address, err := sut.MacAddress()
		Expect(err).NotTo(HaveOccurred())
		Expect(address).To(Equal("00:aa:bb:cc:dd:ee"))

		sut.configuredMAC = "00:11:22:33:44:55"
		again, err := sut.MacAddress()
		Expect(err).NotTo(HaveOccurred())
		Expect(again).To(Equal(address))
	})

	DescribeTable("fails closed on an invalid configured identity", func(value string) {
		sut := New(Args{DeviceMACAddress: value}).(*machine)
		sut.interfaces = func() ([]net.Interface, error) {
			Fail("invalid configured identity must not fall back")

			return nil, nil
		}

		address, err := sut.MacAddress()
		Expect(address).To(BeEmpty())
		Expect(err).To(MatchError("configured device MAC address must be a usable six-byte unicast address"))
	},
		Entry("malformed", "not-a-mac"),
		Entry("whitespace", "  "),
		Entry("multicast", "01:11:22:33:44:55"),
		Entry("zero", "00:00:00:00:00:00"),
		Entry("redacted", "02:00:00:00:00:00"),
		Entry("EUI-64", "00:11:22:33:44:55:66:77"),
	)

	It("retains automatic interface selection without an override", func() {
		sut := New(Args{}).(*machine)
		sut.interfaces = func() ([]net.Interface, error) {
			return []net.Interface{{Name: "test-container", HardwareAddr: net.HardwareAddr{0, 1, 2, 3, 4, 5}}}, nil
		}

		address, err := sut.MacAddress()
		Expect(err).NotTo(HaveOccurred())
		Expect(address).To(Equal("00:01:02:03:04:05"))
	})
})
