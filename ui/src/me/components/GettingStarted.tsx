// Libraries
import React, {FunctionComponent, useState} from 'react'
import {withRouter, WithRouterProps} from 'react-router'
import {connect} from 'react-redux'

// Components
import {
  Panel,
  Button,
  Gradients,
  InfluxColors,
  ComponentColor,
  ComponentSize,
} from '@influxdata/clockface'
import CollectorGraphic from 'src/me/graphics/CollectorGraphic'
import DashboardingGraphic from 'src/me/graphics/DashboardingGraphic'
import ExploreGraphic from 'src/me/graphics/ExploreGraphic'

// Types
import {AppState} from 'src/types'

interface StateProps {
  orgID: string
}

type Props = WithRouterProps & StateProps

const GettingStarted: FunctionComponent<Props> = ({orgID, router}) => {
  const [loadDataAnimating, setLoadDataAnimation] = useState<boolean>(false)
  const handleLoadDataClick = (): void => {
    router.push(`/orgs/${orgID}/load-data/telegrafs`)
  }
  const handleLoadDataMouseOver = (): void => {
    setLoadDataAnimation(true)
  }
  const handleLoadDataMouseOut = (): void => {
    setLoadDataAnimation(false)
  }

  const [dashboardingAnimating, setDashboardingAnimation] = useState<boolean>(
    false
  )
  const handleDashboardsClick = (): void => {
    router.push(`/orgs/${orgID}/dashboards`)
  }
  const handleDashboardsMouseOver = (): void => {
    setDashboardingAnimation(true)
  }
  const handleDashboardsMouseOut = (): void => {
    setDashboardingAnimation(false)
  }

  const [alertsAnimating, setAlertsAnimation] = useState<boolean>(false)
  const handleAlertsClick = (): void => {
    router.push(`/orgs/${orgID}/alerting`)
  }
  const handleAlertsMouseOver = (): void => {
    setAlertsAnimation(true)
  }
  const handleAlertsMouseOut = (): void => {
    setAlertsAnimation(false)
  }

  return (
    <div className="getting-started">
      <Panel
        className="getting-started--card highlighted"
        gradient={Gradients.PolarExpress}
      >
        <div className="getting-started--card-digit">1</div>
        <Panel.Body>
          <CollectorGraphic animate={loadDataAnimating} />
        </Panel.Body>
        <Panel.Footer>
          <Button
            text="Load your data"
            color={ComponentColor.Primary}
            size={ComponentSize.Large}
            onClick={handleLoadDataClick}
            onMouseOver={handleLoadDataMouseOver}
            onMouseOut={handleLoadDataMouseOut}
          />
        </Panel.Footer>
      </Panel>
      <Panel
        className="getting-started--card"
        backgroundColor={InfluxColors.Onyx}
      >
        <div className="getting-started--card-digit">2</div>
        <Panel.Body>
          <DashboardingGraphic animate={dashboardingAnimating} />
        </Panel.Body>
        <Panel.Footer>
          <Button
            text="Build a dashboard"
            color={ComponentColor.Primary}
            size={ComponentSize.Large}
            onClick={handleDashboardsClick}
            onMouseOver={handleDashboardsMouseOver}
            onMouseOut={handleDashboardsMouseOut}
          />
        </Panel.Footer>
      </Panel>
      <Panel
        className="getting-started--card"
        backgroundColor={InfluxColors.Onyx}
      >
        <div className="getting-started--card-digit">3</div>
        <Panel.Body>
          <ExploreGraphic animate={alertsAnimating} />
        </Panel.Body>
        <Panel.Footer>
          <Button
            text="Set up alerting"
            color={ComponentColor.Primary}
            size={ComponentSize.Large}
            onClick={handleAlertsClick}
            onMouseOver={handleAlertsMouseOver}
            onMouseOut={handleAlertsMouseOut}
          />
        </Panel.Footer>
      </Panel>
    </div>
  )
}

const mstp = ({
  orgs: {
    org: {id},
  },
}: AppState): StateProps => {
  return {
    orgID: id,
  }
}

export default withRouter<{}>(
  connect(
    mstp,
    null
  )(GettingStarted)
)
