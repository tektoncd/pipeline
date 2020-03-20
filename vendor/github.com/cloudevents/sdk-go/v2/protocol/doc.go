/*

Package transport defines interfaces to decouple the client package
from transport implementations.

Most event sender and receiver applications should not use this
package, they should use the client package. This package is for
infrastructure developers implementing new transports, or intermediary
components like importers, channels or brokers.

*/
// TODO: merge these two things ^^ vv
/*
Package bindings contains packages that implement different protocol bindings.

Package binding provides interfaces and helper functions for implementing and using protocol bindings in a uniform way.

Available bindings:

* HTTP (using net/http)
* Kafka (using github.com/Shopify/sarama)
* AMQP (using pack.ag/amqp)

*/

package protocol
