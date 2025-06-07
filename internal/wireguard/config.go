package wireguard

import (
	"encoding/base64"
	"fmt"
	"net"
	"os/exec"
	"strings"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// GenerateKeyPair создает новую пару ключей WireGuard
func GenerateKeyPair() (privateKey, publicKey string, err error) {
	privKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return "", "", err
	}
	return privKey.String(), privKey.PublicKey().String(), nil
}

// GenerateClientConfig создает конфигурацию для клиента WireGuard
func GenerateClientConfig(clientPrivateKey, serverPublicKey, serverEndpoint, allowedIPs, dnsServers, clientIP string) string {
	config := fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = %s
DNS = %s

[Peer]
PublicKey = %s
AllowedIPs = %s
Endpoint = %s
PersistentKeepalive = 25
`, clientPrivateKey, clientIP, dnsServers, serverPublicKey, allowedIPs, serverEndpoint)

	return config
}

// GenerateQRCode создает QR код из конфигурации (возвращает base64)
func GenerateQRCode(config string) (string, error) {
	// Проверяем доступность qrencode
	if _, err := exec.LookPath("qrencode"); err != nil {
		return "", fmt.Errorf("qrencode not found: %w", err)
	}

	// Создаем QR код
	cmd := exec.Command("qrencode", "-t", "PNG", "-o", "-")
	cmd.Stdin = strings.NewReader(config)

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to generate QR code: %w", err)
	}

	// Кодируем в base64
	return base64.StdEncoding.EncodeToString(output), nil
}

// GenerateWireGuardLink создает ссылку для автоматического подключения
func GenerateWireGuardLink(config string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(config))
	return fmt.Sprintf("wireguard://tunnels/add/%s", encoded)
}

// AllocateIP выделяет свободный IP адрес из подсети
func AllocateIP(subnet string, usedIPs []string) (string, error) {
	_, ipNet, err := net.ParseCIDR(subnet)
	if err != nil {
		return "", err
	}

	// Создаем мапу занятых IP
	used := make(map[string]bool)
	for _, ip := range usedIPs {
		used[ip] = true
	}

	// Итерируемся по подсети
	for ip := ipNet.IP.Mask(ipNet.Mask); ipNet.Contains(ip); inc(ip) {
		if !used[ip.String()] && !ip.Equal(ipNet.IP) {
			// Возвращаем IP с маской /32 для клиента
			return fmt.Sprintf("%s/32", ip.String()), nil
		}
	}

	return "", fmt.Errorf("no available IPs in subnet %s", subnet)
}

// inc увеличивает IP адрес на 1
func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

// GetUsedIPs возвращает список используемых IP адресов из устройства
func GetUsedIPs(device *wgtypes.Device) []string {
	var ips []string
	for _, peer := range device.Peers {
		for _, allowedIP := range peer.AllowedIPs {
			ips = append(ips, allowedIP.IP.String())
		}
	}
	return ips
}
