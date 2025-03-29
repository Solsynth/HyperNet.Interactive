package gap

import (
	"fmt"
	"strings"
	"time"

	"git.solsynth.dev/hypernet/nexus/pkg/nex"
	"git.solsynth.dev/hypernet/nexus/pkg/nex/cachekit"
	"git.solsynth.dev/hypernet/pusher/pkg/pushkit/pushcon"
	"github.com/samber/lo"

	"git.solsynth.dev/hypernet/nexus/pkg/proto"
	"github.com/rs/zerolog/log"

	"github.com/spf13/viper"
)

var (
	Nx *nex.Conn
	Px *pushcon.Conn
	Ca *cachekit.CaConn
)

func InitializeToNexus() error {
	grpcBind := strings.SplitN(viper.GetString("grpc_bind"), ":", 2)
	httpBind := strings.SplitN(viper.GetString("bind"), ":", 2)

	outboundIp, _ := nex.GetOutboundIP()

	grpcOutbound := fmt.Sprintf("%s:%s", outboundIp, grpcBind[1])
	httpOutbound := fmt.Sprintf("%s:%s", outboundIp, httpBind[1])

	var err error
	Nx, err = nex.NewNexusConn(viper.GetString("nexus_addr"), &proto.ServiceInfo{
		Id:       viper.GetString("id"),
		Type:     "co",
		Label:    "Interactive",
		GrpcAddr: grpcOutbound,
		HttpAddr: lo.ToPtr("http://" + httpOutbound + "/api"),
	})
	if err == nil {
		go func() {
			err := Nx.RunRegistering()
			if err != nil {
				log.Error().Err(err).Msg("An error occurred while registering service...")
			}
		}()
	}

	Px, err = pushcon.NewConn(Nx)
	if err != nil {
		return fmt.Errorf("error during initialize pushcon: %v", err)
	}

	Ca, err = cachekit.NewCaConn(Nx, 3*time.Second)
	if err != nil {
		return fmt.Errorf("error during initialize cachekit: %v", err)
	}

	return err
}
