package fake

type Log struct {
	PodName    string
	Containers []Container
}

func Logs(logs ...Log) []Log {
	ret := []Log{}
	for _, l := range logs {
		ret = append(logs, l)
	}
	return ret
}

func Task(name string, containers ...Container) Log {
	return Log{
		PodName:    name,
		Containers: containers,
	}
}

func Step(name string, logs ...string) Container {
	return Container{
		Name: name,
		Logs: logs,
	}
}
