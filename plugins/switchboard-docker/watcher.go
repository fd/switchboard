package main

import (
	"log"
	"path/filepath"
	"strings"

	"github.com/fd/switchboard/pkg/api/protocol"
	"github.com/fd/switchboard/pkg/plugin"
	"github.com/fsouza/go-dockerclient"
	"golang.org/x/net/context"
)

type controller struct {
	docker *docker.Client
	plugin *plugin.Plugin
	idMap  map[string]string
}

func watch(ctx context.Context, plugin *plugin.Plugin) error {
	var config struct {
		Host      string `hcl:"host"`
		VerifyTLS bool   `hcl:"verify-tls"`
		CertPath  string `hcl:"cert-path"`
	}

	err := plugin.ParseConfig(&config)
	if err != nil {
		return err
	}

	var client *docker.Client
	if config.VerifyTLS {
		cert := filepath.Join(config.CertPath, "cert.pem")
		key := filepath.Join(config.CertPath, "key.pem")
		ca := filepath.Join(config.CertPath, "ca.pem")
		client, err = docker.NewTLSClient(config.Host, cert, key, ca)
	} else {
		client, err = docker.NewClient(config.Host)
	}
	if err != nil {
		return err
	}

	err = client.Ping()
	if err != nil {
		return err
	}

	events := make(chan *docker.APIEvents)

	err = client.AddEventListener(events)
	if err != nil {
		return err
	}

	ctrl := &controller{
		docker: client,
		plugin: plugin,
		idMap:  make(map[string]string),
	}

	addExistingHosts(ctx, ctrl)

	for {
		select {
		case event, open := <-events:
			if !open {
				panic("closed")
			}

			log.Printf("status=%s id=%s name=%s", event.Status, event.ID, event.From)

			go handleEvent(ctx, ctrl, event)

		case <-ctx.Done():
			return nil
		}
	}
}

func handleEvent(ctx context.Context, ctrl *controller, event *docker.APIEvents) {
	switch event.Status {

	case "create":
		// add host
		addHost(ctx, ctrl, event.ID)

	case "destroy":
		// remove host
		removeHost(ctx, ctrl, event.ID)

	case "start":
		// bring host up
		bringHostUp(ctx, ctrl, event.ID)
	case "stop", "die":
		// take host down
		bringHostDown(ctx, ctrl, event.ID)

	}
}

func addExistingHosts(ctx context.Context, ctrl *controller) {
	opts := docker.ListContainersOptions{All: true}
	list, err := ctrl.docker.ListContainers(opts)
	if err != nil {
		log.Printf("error: %s", err)
		return
	}

	for _, container := range list {
		addHost(ctx, ctrl, container.ID)
	}
}

func addHost(ctx context.Context, ctrl *controller, containerID string) {
	info, err := ctrl.docker.InspectContainer(containerID)
	if err != nil {
		log.Printf("error: %s", err)
		return
	}

	name := info.Name
	name = strings.Replace(info.Name, "_", "-", -1)
	name = strings.Trim(name, "/")

	in := protocol.HostAddReq{
		Name:         "docker/" + name,
		AllocateIPv4: true,
	}

	out, err := ctrl.plugin.Hosts().Add(ctx, &in)
	if err != nil {
		log.Printf("error: %s", err)
		return
	}

	ctrl.idMap[info.ID] = out.Host.Id
	log.Printf("added host: %s", out.Host.Name)

	if info.State.Running {
		bringHostUp(ctx, ctrl, containerID)
	}
}

func removeHost(ctx context.Context, ctrl *controller, containerID string) {
	id := ctrl.idMap[containerID]
	if id == "" {
		return
	}

	in := protocol.HostRemoveReq{
		Id: id,
	}

	_, err := ctrl.plugin.Hosts().Remove(ctx, &in)
	if err != nil {
		log.Printf("error: %s", err)
		return
	}

	log.Printf("removed host: %s", in.Id)
}

func bringHostUp(ctx context.Context, ctrl *controller, containerID string) {
	addRules(ctx, ctrl, containerID)

	id := ctrl.idMap[containerID]
	if id == "" {
		return
	}

	in := protocol.HostSetStatusReq{
		Id: id,
		Up: true,
	}

	_, err := ctrl.plugin.Hosts().SetStatus(ctx, &in)
	if err != nil {
		log.Printf("error: %s", err)
		return
	}
}

func bringHostDown(ctx context.Context, ctrl *controller, containerID string) {
	id := ctrl.idMap[containerID]
	if id == "" {
		return
	}

	in := protocol.HostSetStatusReq{
		Id: id,
		Up: false,
	}

	_, err := ctrl.plugin.Hosts().SetStatus(ctx, &in)
	if err != nil {
		log.Printf("error: %s", err)
		return
	}

	clearRules(ctx, ctrl, containerID)
}

func addRules(ctx context.Context, ctrl *controller, containerID string) {
	clearRules(ctx, ctrl, containerID)

	id := ctrl.idMap[containerID]
	if id == "" {
		return
	}

	info, err := ctrl.docker.InspectContainer(containerID)
	if err != nil {
		log.Printf("error: %s", err)
		return
	}

	for _, mapping := range info.NetworkSettings.PortMappingAPI() {
		var proto = protocol.Protocol_UNSET
		switch mapping.Type {
		case "tcp", "TCP":
			proto = protocol.Protocol_TCP
		case "udp", "UDP":
			proto = protocol.Protocol_UDP
		}

		in := protocol.RuleAddReq{
			Protocol:  proto,
			SrcHostId: id,
			SrcPort:   int32(mapping.PrivatePort),
			DstIp:     "192.168.99.100",
			DstPort:   int32(mapping.PublicPort),
		}

		_, err := ctrl.plugin.Rules().Add(ctx, &in)
		if err != nil {
			log.Printf("error: %s", err)
			return
		}
	}
}

func clearRules(ctx context.Context, ctrl *controller, containerID string) {
	id := ctrl.idMap[containerID]
	if id == "" {
		return
	}

	in := protocol.RuleClearReq{
		HostId: id,
	}

	_, err := ctrl.plugin.Rules().Clear(ctx, &in)
	if err != nil {
		log.Printf("error: %s", err)
		return
	}
}
