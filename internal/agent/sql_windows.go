//go:build windows

package agent

import (
	"golang.org/x/sys/windows/registry"
)

const sqlServerRegRoot = `SOFTWARE\Microsoft\Microsoft SQL Server`

// listLocalSqlInstances reads every installed SQL Server instance from the registry. This
// needs no SQL credentials and works whether or not the instance is running, which is
// what lets a stopped instance still show up as stopped rather than vanish.
func listLocalSqlInstances() []localSqlInstance {
	names, err := registry.OpenKey(registry.LOCAL_MACHINE, sqlServerRegRoot+`\Instance Names\SQL`, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return nil
	}
	defer names.Close()

	instanceNames, err := names.ReadValueNames(0)
	if err != nil {
		return nil
	}

	instances := make([]localSqlInstance, 0, len(instanceNames))
	for _, name := range instanceNames {
		id, _, err := names.GetStringValue(name)
		if err != nil {
			continue
		}
		base := sqlServerRegRoot + `\` + id

		inst := localSqlInstance{Name: name}

		// PatchLevel carries the CU-level build; Version stops at the RTM build.
		inst.Version = regString(base+`\Setup`, "PatchLevel")
		if inst.Version == "" {
			inst.Version = regString(base+`\Setup`, "Version")
		}
		inst.Edition = regString(base+`\Setup`, "Edition")

		// The Cluster key exists only for a failover cluster instance.
		if k, err := registry.OpenKey(registry.LOCAL_MACHINE, base+`\Cluster`, registry.QUERY_VALUE|registry.WOW64_64KEY); err == nil {
			inst.Clustered = true
			inst.VirtualServerName, _, _ = k.GetStringValue("ClusterName")
			k.Close()
		}

		tcp := base + `\MSSQLServer\SuperSocketNetLib\Tcp\IPAll`
		inst.Port = regString(tcp, "TcpPort")
		if inst.Port == "" {
			inst.Port = regString(tcp, "TcpDynamicPorts")
		}

		instances = append(instances, inst)
	}
	return instances
}

func regString(path, value string) string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, path, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return ""
	}
	defer k.Close()
	v, _, err := k.GetStringValue(value)
	if err != nil {
		return ""
	}
	return v
}
