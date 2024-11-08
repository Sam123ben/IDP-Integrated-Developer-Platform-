import React from "react";
import Card from "./Card";
import "../styles/TileContainer.css";

function TileContainer({ environments }) {
    return (
        <div className="tile-container">
            {environments && environments.length > 0 ? (
                environments.map((env, index) => (
                    <Card
                        key={index}
                        name={env.name}
                        lastUpdated={env.lastUpdated}
                        status={env.status}
                        contact={env.contact}
                        appVersion={env.appVersion}
                        dbVersion={env.dbVersion}
                        comments={env.comments}
                        statusClass={env.statusClass}
                    />
                ))
            ) : (
                <p>Select an environment to view details.</p>
            )}
        </div>
    );
}

export default TileContainer;