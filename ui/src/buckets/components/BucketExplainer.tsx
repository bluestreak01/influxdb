// Libraries
import React, {FunctionComponent} from 'react'

// Components
import {Panel, InfluxColors} from '@influxdata/clockface'

const TelegrafExplainer: FunctionComponent = () => (
  <Panel backgroundColor={InfluxColors.Onyx} style={{marginTop: '32px'}}>
    <Panel.Header>
      <Panel.Title>What is a Bucket?</Panel.Title>
    </Panel.Header>
    <Panel.Body>
      <p>
        A bucket is a named location where time series data is stored. All
        buckets have a <b>Retention Policy</b>, a duration of time that each
        data point persists.
        <br />
        <br />
        Here's{' '}
        <a
          href="https://v2.docs.influxdata.com/v2.0/write-data/"
          target="_blank"
        >
          how to write data
        </a>{' '}
        into your bucket.
      </p>
    </Panel.Body>
  </Panel>
)

export default TelegrafExplainer
