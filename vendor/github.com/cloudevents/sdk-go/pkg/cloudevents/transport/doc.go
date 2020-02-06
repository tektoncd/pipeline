/*

Package transport defines interfaces to decouple the client package
from transport implementations.

Most event sender and receiver applications should not use this
package, they should use the client package. This package is for
infrastructure developers implementing new transports, or intermediary
components like importers, channels or brokers.

*/
package transport
